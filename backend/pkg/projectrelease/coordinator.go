package projectrelease

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"
)

const (
	JobStatusRunning   = "running"
	JobStatusCompleted = "completed"
)

// ErrReleaseInProgress is returned when another auto-release is already running.
var ErrReleaseInProgress = errors.New("auto release already in progress")

// BatchReleaseJob tracks one workspace auto-release run.
type BatchReleaseJob struct {
	ReleaseRecID    string          `json:"release_rec_id,omitempty"`
	WorkspaceRecID  string          `json:"workspace_rec_id"`
	WorkspaceName   string          `json:"workspace_name,omitempty"`
	Status          string          `json:"status"`
	Tag             string          `json:"tag,omitempty"`
	ReleaseBranch   string          `json:"release_branch,omitempty"`
	StartedAt       time.Time       `json:"started_at"`
	FinishedAt      *time.Time      `json:"finished_at,omitempty"`
	Released        int             `json:"released"`
	Skipped         int             `json:"skipped"`
	Failed          int             `json:"failed"`
	Results         []ReleaseResult `json:"results,omitempty"`
	CurrentProject  string          `json:"current_project,omitempty"`
	DoneCount       int             `json:"done_count"`
	TotalCount      int             `json:"total_count"`
	Foreign         bool            `json:"foreign,omitempty"`
	ActiveWorkspace string          `json:"active_workspace_name,omitempty"`
}

// BatchReleaseStatus is the API view for poll / dialog open.
type BatchReleaseStatus struct {
	BatchReleaseJob
}

// Coordinator ensures at most one auto-release runs at a time (global lock).
type Coordinator struct {
	mu  sync.Mutex
	job *BatchReleaseJob
}

func NewCoordinator() *Coordinator {
	return &Coordinator{}
}

// Start begins an async release unless another job is running.
func (c *Coordinator) Start(workspaceRecID, workspaceName string, total int, meta BatchReleaseJob, run func(ctx context.Context, job *BatchReleaseJob)) error {
	workspaceRecID = trimID(workspaceRecID)
	if workspaceRecID == "" {
		return errors.New("workspace_rec_id is required")
	}
	c.mu.Lock()
	if c.job != nil && c.job.Status == JobStatusRunning {
		err := ErrReleaseInProgress
		c.mu.Unlock()
		return err
	}
	now := time.Now().UTC()
	j := &BatchReleaseJob{
		ReleaseRecID:   meta.ReleaseRecID,
		WorkspaceRecID: workspaceRecID,
		WorkspaceName:  workspaceName,
		Status:         JobStatusRunning,
		Tag:            meta.Tag,
		ReleaseBranch:  meta.ReleaseBranch,
		StartedAt:      now,
		TotalCount:     total,
	}
	c.job = j
	c.mu.Unlock()

	go func() {
		ctx := context.Background()
		run(ctx, j)
		c.mu.Lock()
		fin := time.Now().UTC()
		j.FinishedAt = &fin
		j.Status = JobStatusCompleted
		j.CurrentProject = ""
		c.mu.Unlock()
	}()
	return nil
}

// Status returns the current job snapshot for polling. queryWorkspace is the dialog's workspace.
func (c *Coordinator) Status(queryWorkspace string) BatchReleaseStatus {
	queryWorkspace = trimID(queryWorkspace)
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.job == nil {
		return BatchReleaseStatus{BatchReleaseJob: BatchReleaseJob{Status: "idle"}}
	}
	j := *c.job
	if len(j.Results) > 0 {
		j.Results = append([]ReleaseResult(nil), j.Results...)
	}
	if j.Status == JobStatusRunning && j.WorkspaceRecID != queryWorkspace {
		return BatchReleaseStatus{BatchReleaseJob: BatchReleaseJob{
			ReleaseRecID:    j.ReleaseRecID,
			Status:          JobStatusRunning,
			Foreign:         true,
			ActiveWorkspace: j.WorkspaceName,
			Tag:             j.Tag,
			ReleaseBranch:   j.ReleaseBranch,
			StartedAt:       j.StartedAt,
			DoneCount:       j.DoneCount,
			TotalCount:      j.TotalCount,
			CurrentProject:  j.CurrentProject,
		}}
	}
	return BatchReleaseStatus{BatchReleaseJob: j}
}

// SetProgress updates running job progress (call from release loop).
func (c *Coordinator) SetProgress(current string, done int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.job == nil || c.job.Status != JobStatusRunning {
		return
	}
	c.job.CurrentProject = current
	c.job.DoneCount = done
}

// AppendResult records one finished project (safe to call while running).
func (c *Coordinator) AppendResult(res ReleaseResult, released, skipped, failed int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.job == nil || c.job.Status != JobStatusRunning {
		return
	}
	c.job.Results = append(c.job.Results, res)
	c.job.Released = released
	c.job.Skipped = skipped
	c.job.Failed = failed
	c.job.DoneCount = len(c.job.Results)
}

// FinishResult stores aggregate outcome on the running job.
func (c *Coordinator) FinishResult(released, skipped, failed int, results []ReleaseResult) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.job == nil {
		return
	}
	c.job.Released = released
	c.job.Skipped = skipped
	c.job.Failed = failed
	c.job.Results = append([]ReleaseResult(nil), results...)
	c.job.DoneCount = len(c.job.Results)
}

func trimID(s string) string {
	return strings.TrimSpace(s)
}
