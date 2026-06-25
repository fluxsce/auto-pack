package projectrelease

import (
	"context"
	"errors"
	"sync"
	"time"
)

// ErrPomVersionInProgress is returned when another pom version batch is already running.
var ErrPomVersionInProgress = errors.New("pom version batch already in progress")

// BatchPomVersionJob tracks one workspace pom version batch run.
type BatchPomVersionJob struct {
	BatchRecID      string             `json:"batch_rec_id,omitempty"`
	WorkspaceRecID  string             `json:"workspace_rec_id"`
	WorkspaceName   string             `json:"workspace_name,omitempty"`
	Status          string             `json:"status"`
	Tag             string             `json:"tag,omitempty"`
	CommitMessage   string             `json:"commit_message,omitempty"`
	StartedAt       time.Time          `json:"started_at"`
	FinishedAt      *time.Time         `json:"finished_at,omitempty"`
	Updated         int                `json:"updated"`
	Skipped         int                `json:"skipped"`
	Failed          int                `json:"failed"`
	Results         []PomVersionResult `json:"results,omitempty"`
	CurrentProject  string             `json:"current_project,omitempty"`
	DoneCount       int                `json:"done_count"`
	TotalCount      int                `json:"total_count"`
	Foreign         bool               `json:"foreign,omitempty"`
	ActiveWorkspace string             `json:"active_workspace_name,omitempty"`
}

// BatchPomVersionStatus is the API view for poll / dialog open.
type BatchPomVersionStatus struct {
	BatchPomVersionJob
}

// PomVersionCoordinator ensures at most one pom version batch runs at a time (global lock).
type PomVersionCoordinator struct {
	mu  sync.Mutex
	job *BatchPomVersionJob
}

func NewPomVersionCoordinator() *PomVersionCoordinator {
	return &PomVersionCoordinator{}
}

// Start begins an async pom version batch unless another job is running.
func (c *PomVersionCoordinator) Start(workspaceRecID, workspaceName string, total int, meta BatchPomVersionJob, run func(ctx context.Context, job *BatchPomVersionJob)) error {
	workspaceRecID = trimID(workspaceRecID)
	if workspaceRecID == "" {
		return errors.New("workspace_rec_id is required")
	}
	c.mu.Lock()
	if c.job != nil && c.job.Status == JobStatusRunning {
		err := ErrPomVersionInProgress
		c.mu.Unlock()
		return err
	}
	now := time.Now().UTC()
	j := &BatchPomVersionJob{
		BatchRecID:     meta.BatchRecID,
		WorkspaceRecID: workspaceRecID,
		WorkspaceName:  workspaceName,
		Status:         JobStatusRunning,
		Tag:            meta.Tag,
		CommitMessage:  meta.CommitMessage,
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

// Status returns the current job snapshot for polling.
func (c *PomVersionCoordinator) Status(queryWorkspace string) BatchPomVersionStatus {
	queryWorkspace = trimID(queryWorkspace)
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.job == nil {
		return BatchPomVersionStatus{BatchPomVersionJob: BatchPomVersionJob{Status: "idle"}}
	}
	j := *c.job
	if len(j.Results) > 0 {
		j.Results = append([]PomVersionResult(nil), j.Results...)
	}
	if j.Status == JobStatusRunning && j.WorkspaceRecID != queryWorkspace {
		return BatchPomVersionStatus{BatchPomVersionJob: BatchPomVersionJob{
			BatchRecID:      j.BatchRecID,
			Status:          JobStatusRunning,
			Foreign:         true,
			ActiveWorkspace: j.WorkspaceName,
			Tag:             j.Tag,
			StartedAt:       j.StartedAt,
			DoneCount:       j.DoneCount,
			TotalCount:      j.TotalCount,
			CurrentProject:  j.CurrentProject,
		}}
	}
	return BatchPomVersionStatus{BatchPomVersionJob: j}
}

// SetProgress updates running job progress.
func (c *PomVersionCoordinator) SetProgress(current string, done int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.job == nil || c.job.Status != JobStatusRunning {
		return
	}
	c.job.CurrentProject = current
	c.job.DoneCount = done
}

// AppendResult records one finished project.
func (c *PomVersionCoordinator) AppendResult(res PomVersionResult, updated, skipped, failed int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.job == nil || c.job.Status != JobStatusRunning {
		return
	}
	c.job.Results = append(c.job.Results, res)
	c.job.Updated = updated
	c.job.Skipped = skipped
	c.job.Failed = failed
	c.job.DoneCount = len(c.job.Results)
}

// FinishResult stores aggregate outcome on the running job.
func (c *PomVersionCoordinator) FinishResult(updated, skipped, failed int, results []PomVersionResult) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.job == nil {
		return
	}
	c.job.Updated = updated
	c.job.Skipped = skipped
	c.job.Failed = failed
	c.job.Results = append([]PomVersionResult(nil), results...)
	c.job.DoneCount = len(c.job.Results)
}

// PomVersionResultsAsReleaseResults converts pom results for shared log formatting.
func PomVersionResultsAsReleaseResults(items []PomVersionResult) []ReleaseResult {
	if len(items) == 0 {
		return nil
	}
	out := make([]ReleaseResult, len(items))
	for i, item := range items {
		out[i] = ReleaseResult{
			ProjectName: item.ProjectName,
			Status:      item.Status,
			Message:     item.Message,
			Steps:       item.Steps,
		}
	}
	return out
}
