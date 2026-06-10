package packrun

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"
)

const (
	SyncJobStatusIdle      = "idle"
	SyncJobStatusRunning   = "running"
	SyncJobStatusCompleted = "completed"
)

// ErrWorkspaceSyncInProgress is returned when another workspace source sync is already running.
var ErrWorkspaceSyncInProgress = errors.New("workspace source sync already in progress")

// WorkspaceSourceSyncJob tracks one async git pre-sync run.
type WorkspaceSourceSyncJob struct {
	WorkspaceRecID  string     `json:"workspace_rec_id"`
	WorkspaceName   string     `json:"workspace_name,omitempty"`
	Status          string     `json:"status"`
	CurrentProject  string     `json:"current_project,omitempty"`
	DoneCount       int        `json:"done_count"`
	TotalCount      int        `json:"total_count"`
	Synced          int        `json:"synced"`
	Skipped         int        `json:"skipped"`
	Failed          int        `json:"failed"`
	FailedItems     []string   `json:"failed_items,omitempty"`
	Foreign         bool       `json:"foreign,omitempty"`
	ActiveWorkspace string     `json:"active_workspace_name,omitempty"`
	StartedAt       time.Time  `json:"started_at,omitempty"`
	FinishedAt      *time.Time `json:"finished_at,omitempty"`
}

type workspaceSourceSyncCoordinator struct {
	mu  sync.Mutex
	job *WorkspaceSourceSyncJob
}

func (c *workspaceSourceSyncCoordinator) start(
	workspaceRecID, workspaceName string,
	run func(ctx context.Context, job *WorkspaceSourceSyncJob),
) error {
	workspaceRecID = strings.TrimSpace(workspaceRecID)
	if workspaceRecID == "" {
		return errors.New("workspace_rec_id is required")
	}
	c.mu.Lock()
	if c.job != nil && c.job.Status == SyncJobStatusRunning {
		if c.job.WorkspaceRecID == workspaceRecID {
			c.mu.Unlock()
			return nil
		}
		err := ErrWorkspaceSyncInProgress
		c.mu.Unlock()
		return err
	}
	now := time.Now().UTC()
	j := &WorkspaceSourceSyncJob{
		WorkspaceRecID: workspaceRecID,
		WorkspaceName:  workspaceName,
		Status:         SyncJobStatusRunning,
		StartedAt:      now,
	}
	c.job = j
	c.mu.Unlock()

	go func() {
		ctx := context.Background()
		run(ctx, j)
		c.mu.Lock()
		fin := time.Now().UTC()
		j.FinishedAt = &fin
		j.Status = SyncJobStatusCompleted
		j.CurrentProject = ""
		c.mu.Unlock()
	}()
	return nil
}

func (c *workspaceSourceSyncCoordinator) status(queryWorkspace string) WorkspaceSourceSyncJob {
	queryWorkspace = strings.TrimSpace(queryWorkspace)
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.job == nil {
		return WorkspaceSourceSyncJob{Status: SyncJobStatusIdle}
	}
	j := *c.job
	if len(j.FailedItems) > 0 {
		j.FailedItems = append([]string(nil), j.FailedItems...)
	}
	if j.Status == SyncJobStatusRunning && j.WorkspaceRecID != queryWorkspace {
		return WorkspaceSourceSyncJob{
			WorkspaceRecID:  j.WorkspaceRecID,
			Status:          SyncJobStatusRunning,
			Foreign:         true,
			ActiveWorkspace: j.WorkspaceName,
			StartedAt:       j.StartedAt,
			DoneCount:       j.DoneCount,
			TotalCount:      j.TotalCount,
			CurrentProject:  j.CurrentProject,
			Synced:          j.Synced,
			Skipped:         j.Skipped,
			Failed:          j.Failed,
		}
	}
	return j
}

func (c *workspaceSourceSyncCoordinator) setProgress(current string, done, total int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.job == nil || c.job.Status != SyncJobStatusRunning {
		return
	}
	c.job.CurrentProject = current
	c.job.DoneCount = done
	if total > 0 {
		c.job.TotalCount = total
	}
}

func (c *workspaceSourceSyncCoordinator) setInitialSkipped(skipped int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.job == nil || c.job.Status != SyncJobStatusRunning {
		return
	}
	c.job.Skipped = skipped
}

func (c *workspaceSourceSyncCoordinator) recordOutcome(synced, skipped bool, failMsg string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.job == nil || c.job.Status != SyncJobStatusRunning {
		return
	}
	if failMsg != "" {
		c.job.Failed++
		c.job.FailedItems = append(c.job.FailedItems, failMsg)
	} else if synced {
		c.job.Synced++
	} else if skipped {
		c.job.Skipped++
	}
}
