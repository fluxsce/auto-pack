package packrun

import (
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	SyncJobStatusIdle      = "idle"
	SyncJobStatusRunning   = "running"
	SyncJobStatusCompleted = "completed"

	// DefaultWorkspaceSyncMaxConcurrent allows a few workspaces to pre-sync in
	// parallel without saturating disk/network. Same workspace remains single-flight.
	DefaultWorkspaceSyncMaxConcurrent = 2
)

// ErrWorkspaceSyncCapacity is returned when too many workspace source syncs are running.
var ErrWorkspaceSyncCapacity = errors.New("workspace source sync capacity reached")

// ErrWorkspaceSyncInProgress is kept for callers/tests that still check the old name.
// It unwraps to ErrWorkspaceSyncCapacity (cross-workspace blocking is no longer global).
var ErrWorkspaceSyncInProgress = ErrWorkspaceSyncCapacity

// workspaceSyncMaxConcurrent is process-wide from app.yaml git.sync_max_workspaces.
var workspaceSyncMaxConcurrent atomic.Int64

func init() {
	workspaceSyncMaxConcurrent.Store(int64(DefaultWorkspaceSyncMaxConcurrent))
}

// SetWorkspaceSyncMaxConcurrent applies configs/app.yaml git.sync_max_workspaces.
// Values < 1 fall back to DefaultWorkspaceSyncMaxConcurrent.
func SetWorkspaceSyncMaxConcurrent(n int) {
	if n < 1 {
		n = DefaultWorkspaceSyncMaxConcurrent
	}
	workspaceSyncMaxConcurrent.Store(int64(n))
}

// WorkspaceSyncMaxConcurrent returns the effective global parallel workspace sync limit.
func WorkspaceSyncMaxConcurrent() int {
	return int(workspaceSyncMaxConcurrent.Load())
}

// WorkspaceSourceSyncJob tracks one async git pre-sync run for a single workspace.
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
	Foreign         bool       `json:"foreign,omitempty"`               // true when start was rejected for capacity (status hint)
	ActiveWorkspace string     `json:"active_workspace_name,omitempty"` // comma-joined running workspace names when capacity full
	StartedAt       time.Time  `json:"started_at,omitempty"`
	FinishedAt      *time.Time `json:"finished_at,omitempty"`
}

// workspaceSourceSyncCoordinator tracks per-workspace jobs with a global concurrency cap.
// Different workspaces may sync in parallel up to WorkspaceSyncMaxConcurrent();
// the same workspace is single-flight (repeat start is a no-op while running).
type workspaceSourceSyncCoordinator struct {
	mu   sync.Mutex
	jobs map[string]*WorkspaceSourceSyncJob // workspace_rec_id -> job
}

func (c *workspaceSourceSyncCoordinator) ensureJobs() {
	if c.jobs == nil {
		c.jobs = make(map[string]*WorkspaceSourceSyncJob)
	}
}

func (c *workspaceSourceSyncCoordinator) runningCountLocked() int {
	n := 0
	for _, j := range c.jobs {
		if j != nil && j.Status == SyncJobStatusRunning {
			n++
		}
	}
	return n
}

func (c *workspaceSourceSyncCoordinator) runningNamesLocked() []string {
	var names []string
	for _, j := range c.jobs {
		if j == nil || j.Status != SyncJobStatusRunning {
			continue
		}
		name := strings.TrimSpace(j.WorkspaceName)
		if name == "" {
			name = j.WorkspaceRecID
		}
		names = append(names, name)
	}
	return names
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
	c.ensureJobs()
	if j := c.jobs[workspaceRecID]; j != nil && j.Status == SyncJobStatusRunning {
		// Same workspace: idempotent — do not start a second sync.
		c.mu.Unlock()
		return nil
	}
	max := WorkspaceSyncMaxConcurrent()
	if c.runningCountLocked() >= max {
		c.mu.Unlock()
		return ErrWorkspaceSyncCapacity
	}
	now := time.Now().UTC()
	j := &WorkspaceSourceSyncJob{
		WorkspaceRecID: workspaceRecID,
		WorkspaceName:  workspaceName,
		Status:         SyncJobStatusRunning,
		StartedAt:      now,
	}
	c.jobs[workspaceRecID] = j
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

// RunningSyncSummary returns how many workspace syncs are active, the max, and their names.
func (c *workspaceSourceSyncCoordinator) RunningSyncSummary() (running, max int, names []string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.ensureJobs()
	return c.runningCountLocked(), WorkspaceSyncMaxConcurrent(), c.runningNamesLocked()
}

func (c *workspaceSourceSyncCoordinator) status(queryWorkspace string) WorkspaceSourceSyncJob {
	queryWorkspace = strings.TrimSpace(queryWorkspace)
	c.mu.Lock()
	defer c.mu.Unlock()
	c.ensureJobs()
	if queryWorkspace == "" {
		return WorkspaceSourceSyncJob{Status: SyncJobStatusIdle}
	}
	j := c.jobs[queryWorkspace]
	if j == nil {
		return WorkspaceSourceSyncJob{Status: SyncJobStatusIdle}
	}
	out := *j
	if len(out.FailedItems) > 0 {
		out.FailedItems = append([]string(nil), out.FailedItems...)
	}
	return out
}

func (c *workspaceSourceSyncCoordinator) setProgress(workspaceRecID, current string, done, total int) {
	workspaceRecID = strings.TrimSpace(workspaceRecID)
	c.mu.Lock()
	defer c.mu.Unlock()
	j := c.jobs[workspaceRecID]
	if j == nil || j.Status != SyncJobStatusRunning {
		return
	}
	j.CurrentProject = current
	j.DoneCount = done
	if total > 0 {
		j.TotalCount = total
	}
}

func (c *workspaceSourceSyncCoordinator) setInitialSkipped(workspaceRecID string, skipped int) {
	workspaceRecID = strings.TrimSpace(workspaceRecID)
	c.mu.Lock()
	defer c.mu.Unlock()
	j := c.jobs[workspaceRecID]
	if j == nil || j.Status != SyncJobStatusRunning {
		return
	}
	j.Skipped = skipped
}

func (c *workspaceSourceSyncCoordinator) recordOutcome(workspaceRecID string, synced, skipped bool, failMsg string) {
	workspaceRecID = strings.TrimSpace(workspaceRecID)
	c.mu.Lock()
	defer c.mu.Unlock()
	j := c.jobs[workspaceRecID]
	if j == nil || j.Status != SyncJobStatusRunning {
		return
	}
	if failMsg != "" {
		j.Failed++
		j.FailedItems = append(j.FailedItems, failMsg)
	} else if synced {
		j.Synced++
	} else if skipped {
		j.Skipped++
	}
}
