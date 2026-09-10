// Package record 持久化 AP_WORKSPACE_RELEASE，并提供按工作空间的发版锁与批量日志格式。
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
	Foreign         bool         `json:"foreign,omitempty"`               // leftover API field; status never sets it
	ActiveWorkspace string       `json:"active_workspace_name,omitempty"` // leftover API field; unused after per-workspace lock
	PostOptions     *PostOptions `json:"post_options,omitempty"`
	Phase           string       `json:"phase,omitempty"`
}

// Status is the API view for poll / dialog open.
type Status struct {
	Job
}

// Coordinator 按工作空间单飞：同一空间同时只跑一次发版，不同空间互不影响。
type Coordinator struct {
	mu   sync.Mutex
	jobs map[string]*Job
}

// NewCoordinator constructs a per-workspace single-flight Coordinator.
func NewCoordinator() *Coordinator {
	return &Coordinator{jobs: map[string]*Job{}}
}

// Start begins an async release unless that workspace already has one running.
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
	if c.jobs == nil {
		c.jobs = map[string]*Job{}
	}
	if existing := c.jobs[workspaceRecID]; existing != nil && existing.Status == JobStatusRunning {
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
	c.jobs[workspaceRecID] = j
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

// Status returns this workspace's job snapshot. Other workspaces' runs are invisible.
func (c *Coordinator) Status(queryWorkspace string) Status {
	queryWorkspace = trimID(queryWorkspace)
	c.mu.Lock()
	defer c.mu.Unlock()
	j := c.jobs[queryWorkspace]
	if j == nil {
		return Status{Job: Job{Status: JobStatusIdle}}
	}
	out := *j
	if len(out.Results) > 0 {
		out.Results = append([]Result(nil), out.Results...)
	}
	return Status{Job: out}
}

// SetProgress updates running job progress (call from release loop).
func (c *Coordinator) SetProgress(job *Job, current string, done int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if job == nil || job.Status != JobStatusRunning {
		return
	}
	job.CurrentProject = current
	job.DoneCount = done
}

// AppendResult records one finished project (safe to call while running).
func (c *Coordinator) AppendResult(job *Job, res Result, released, skipped, failed int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if job == nil || job.Status != JobStatusRunning {
		return
	}
	job.Results = append(job.Results, res)
	job.Released = released
	job.Skipped = skipped
	job.Failed = failed
	job.DoneCount = len(job.Results)
}

// FinishResult stores aggregate outcome on the running job.
func (c *Coordinator) FinishResult(job *Job, released, skipped, failed int, results []Result) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if job == nil {
		return
	}
	job.Released = released
	job.Skipped = skipped
	job.Failed = failed
	job.Results = append([]Result(nil), results...)
	job.DoneCount = len(job.Results)
}

// SetPhase records whether the run is still releasing or already in post-build / push.
func (c *Coordinator) SetPhase(job *Job, phase string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if job == nil || job.Status != JobStatusRunning {
		return
	}
	job.Phase = phase
}

// SetPostOptions updates the in-memory job so poll/status stay aligned with the DB row.
func (c *Coordinator) SetPostOptions(workspaceID string, opts *PostOptions) {
	c.mu.Lock()
	defer c.mu.Unlock()
	j := c.jobs[trimID(workspaceID)]
	if j == nil {
		return
	}
	j.PostOptions = opts
}

func trimID(s string) string {
	return strings.TrimSpace(s)
}
