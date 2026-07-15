package projectrelease

import (
	"context"
	"errors"
	"sync"
	"time"
)

// ErrBranchCloneInProgress is returned when another branch clone batch is already running.
var ErrBranchCloneInProgress = errors.New("branch clone batch already in progress")

// BranchCloneResult is the outcome for one project.
type BranchCloneResult struct {
	ProjectName string   `json:"project_name"`
	Status      string   `json:"status"`
	Message     string   `json:"message,omitempty"`
	Steps       []string `json:"steps,omitempty"`
}

// BatchBranchCloneJob tracks one workspace branch clone batch run.
type BatchBranchCloneJob struct {
	BatchRecID      string              `json:"batch_rec_id,omitempty"`
	WorkspaceRecID  string              `json:"workspace_rec_id"`
	WorkspaceName   string              `json:"workspace_name,omitempty"`
	Status          string              `json:"status"`
	SourceBranch    string              `json:"source_branch,omitempty"`
	TargetBranch    string              `json:"target_branch,omitempty"`
	StartedAt       time.Time           `json:"started_at"`
	FinishedAt      *time.Time          `json:"finished_at,omitempty"`
	Cloned          int                 `json:"cloned"`
	Skipped         int                 `json:"skipped"`
	Failed          int                 `json:"failed"`
	Results         []BranchCloneResult `json:"results,omitempty"`
	CurrentProject  string              `json:"current_project,omitempty"`
	DoneCount       int                 `json:"done_count"`
	TotalCount      int                 `json:"total_count"`
	Foreign         bool                `json:"foreign,omitempty"`
	ActiveWorkspace string              `json:"active_workspace_name,omitempty"`
}

// BatchBranchCloneStatus is the API view for poll / dialog open.
type BatchBranchCloneStatus struct {
	BatchBranchCloneJob
}

// BranchCloneCoordinator ensures at most one branch clone batch runs at a time (global lock).
type BranchCloneCoordinator struct {
	mu  sync.Mutex
	job *BatchBranchCloneJob
}

func NewBranchCloneCoordinator() *BranchCloneCoordinator {
	return &BranchCloneCoordinator{}
}

// Start begins an async branch clone batch unless another job is running.
func (c *BranchCloneCoordinator) Start(
	workspaceRecID, workspaceName string,
	total int,
	meta BatchBranchCloneJob,
	run func(ctx context.Context, job *BatchBranchCloneJob),
) error {
	workspaceRecID = trimID(workspaceRecID)
	if workspaceRecID == "" {
		return errors.New("workspace_rec_id is required")
	}
	c.mu.Lock()
	if c.job != nil && c.job.Status == JobStatusRunning {
		err := ErrBranchCloneInProgress
		c.mu.Unlock()
		return err
	}
	now := time.Now().UTC()
	j := &BatchBranchCloneJob{
		BatchRecID:     meta.BatchRecID,
		WorkspaceRecID: workspaceRecID,
		WorkspaceName:  workspaceName,
		Status:         JobStatusRunning,
		SourceBranch:   meta.SourceBranch,
		TargetBranch:   meta.TargetBranch,
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
func (c *BranchCloneCoordinator) Status(queryWorkspace string) BatchBranchCloneStatus {
	queryWorkspace = trimID(queryWorkspace)
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.job == nil {
		return BatchBranchCloneStatus{BatchBranchCloneJob: BatchBranchCloneJob{Status: "idle"}}
	}
	j := *c.job
	if len(j.Results) > 0 {
		j.Results = append([]BranchCloneResult(nil), j.Results...)
	}
	if j.Status == JobStatusRunning && j.WorkspaceRecID != queryWorkspace {
		return BatchBranchCloneStatus{BatchBranchCloneJob: BatchBranchCloneJob{
			BatchRecID:      j.BatchRecID,
			Status:          JobStatusRunning,
			Foreign:         true,
			ActiveWorkspace: j.WorkspaceName,
			SourceBranch:    j.SourceBranch,
			TargetBranch:    j.TargetBranch,
			StartedAt:       j.StartedAt,
			DoneCount:       j.DoneCount,
			TotalCount:      j.TotalCount,
			CurrentProject:  j.CurrentProject,
		}}
	}
	return BatchBranchCloneStatus{BatchBranchCloneJob: j}
}

// SetProgress updates running job progress.
func (c *BranchCloneCoordinator) SetProgress(current string, done int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.job == nil || c.job.Status != JobStatusRunning {
		return
	}
	c.job.CurrentProject = current
	c.job.DoneCount = done
}

// AppendResult records one finished project.
func (c *BranchCloneCoordinator) AppendResult(res BranchCloneResult, cloned, skipped, failed int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.job == nil || c.job.Status != JobStatusRunning {
		return
	}
	c.job.Results = append(c.job.Results, res)
	c.job.Cloned = cloned
	c.job.Skipped = skipped
	c.job.Failed = failed
	c.job.DoneCount = len(c.job.Results)
}

// FinishResult stores aggregate outcome on the running job.
func (c *BranchCloneCoordinator) FinishResult(cloned, skipped, failed int, results []BranchCloneResult) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.job == nil {
		return
	}
	c.job.Cloned = cloned
	c.job.Skipped = skipped
	c.job.Failed = failed
	c.job.Results = append([]BranchCloneResult(nil), results...)
	c.job.DoneCount = len(c.job.Results)
}

// BranchCloneResultsAsReleaseResults converts clone results for shared log formatting.
func BranchCloneResultsAsReleaseResults(items []BranchCloneResult) []ReleaseResult {
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
