// Package record 持久化 AP_WORKSPACE_RELEASE，并提供进程内全局发版锁与批量日志格式。
package record

import (
	"context"
	"errors"
	"log"
	"runtime/debug"
	"strings"
	"sync"
	"time"
)

const (
	// JobStatusIdle means no batch is tracked.
	JobStatusIdle = "idle"
	// JobStatusRunning is the in-memory coordinator status while a batch is active.
	JobStatusRunning = "running"
	// JobStatusCompleted is the in-memory coordinator status after the batch goroutine finishes.
	JobStatusCompleted = "completed"
)

// Job tracks one workspace auto-release run.
type Job struct {
	ReleaseRecID    string       `json:"release_rec_id,omitempty"`
	WorkspaceRecID  string       `json:"workspace_rec_id"`
	WorkspaceName   string       `json:"workspace_name,omitempty"`
	Status          string       `json:"status"`
	Tag             string       `json:"tag,omitempty"`
	ReleaseBranch   string       `json:"release_branch,omitempty"`
	StartedAt       time.Time    `json:"started_at"`
	FinishedAt      *time.Time   `json:"finished_at,omitempty"`
	Released        int          `json:"released"`
	Skipped         int          `json:"skipped"`
	Failed          int          `json:"failed"`
	Results         []Result     `json:"results,omitempty"`
	CurrentProject  string       `json:"current_project,omitempty"`
	DoneCount       int          `json:"done_count"`
	TotalCount      int          `json:"total_count"`
	Foreign         bool         `json:"foreign,omitempty"`
	ActiveWorkspace string       `json:"active_workspace_name,omitempty"`
	PostOptions     *PostOptions `json:"post_options,omitempty"`
}

// Status is the API view for poll / dialog open.
type Status struct {
	Job
}

// Coordinator ensures at most one auto-release runs at a time (global lock).
type Coordinator struct {
	mu  sync.Mutex
	job *Job
}

// NewCoordinator constructs a process-wide single-flight Coordinator.
func NewCoordinator() *Coordinator {
	return &Coordinator{}
}

// Start begins an async release unless another job is running.
// ctx should be the process lifetime; a nil ctx uses context.Background.
func (c *Coordinator) Start(ctx context.Context, workspaceRecID, workspaceName string, total int, meta Job, run func(ctx context.Context, job *Job)) error {
	workspaceRecID = trimID(workspaceRecID)
	if workspaceRecID == "" {
		return errors.New("workspace_rec_id is required")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	c.mu.Lock()
	if c.job != nil && c.job.Status == JobStatusRunning {
		c.mu.Unlock()
		return ErrReleaseInProgress
	}
	now := time.Now().UTC()
	j := &Job{
		ReleaseRecID:   meta.ReleaseRecID,
		WorkspaceRecID: workspaceRecID,
		WorkspaceName:  workspaceName,
		Status:         JobStatusRunning,
		Tag:            meta.Tag,
		ReleaseBranch:  meta.ReleaseBranch,
		StartedAt:      now,
		TotalCount:     total,
		PostOptions:    meta.PostOptions,
	}
	c.job = j
	c.mu.Unlock()

	go func() {
		defer func() {
			if rec := recover(); rec != nil {
				log.Printf("release coordinator panic workspace=%s: %v\n%s", workspaceRecID, rec, debug.Stack())
			}
			c.mu.Lock()
			fin := time.Now().UTC()
			j.FinishedAt = &fin
			j.Status = JobStatusCompleted
			j.CurrentProject = ""
			c.mu.Unlock()
		}()
		run(ctx, j)
	}()
	return nil
}

// Status returns the current job snapshot for polling. queryWorkspace is the dialog's workspace.
func (c *Coordinator) Status(queryWorkspace string) Status {
	queryWorkspace = trimID(queryWorkspace)
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.job == nil {
		return Status{Job: Job{Status: JobStatusIdle}}
	}
	j := *c.job
	if len(j.Results) > 0 {
		j.Results = append([]Result(nil), j.Results...)
	}
	if j.Status == JobStatusRunning && j.WorkspaceRecID != queryWorkspace {
		return Status{Job: Job{
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
	return Status{Job: j}
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
func (c *Coordinator) AppendResult(res Result, released, skipped, failed int) {
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
func (c *Coordinator) FinishResult(released, skipped, failed int, results []Result) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.job == nil {
		return
	}
	c.job.Released = released
	c.job.Skipped = skipped
	c.job.Failed = failed
	c.job.Results = append([]Result(nil), results...)
	c.job.DoneCount = len(c.job.Results)
}

// SetPostOptions updates the in-memory job so poll/status stay aligned with the DB row.
func (c *Coordinator) SetPostOptions(opts *PostOptions) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.job == nil {
		return
	}
	c.job.PostOptions = opts
}

func trimID(s string) string {
	return strings.TrimSpace(s)
}
