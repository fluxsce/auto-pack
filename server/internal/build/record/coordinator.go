// Package record 持久化 AP_WORKSPACE_BUILD，并提供按工作空间的自动构建锁。
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

// Result is one finished project in an auto-build batch.
type Result struct {
	ProjectName string `json:"project_name"`
	Status      string `json:"status"`
	Message     string `json:"message,omitempty"`
}

// Job tracks one workspace auto-build run.
type Job struct {
	BuildRecID     string     `json:"build_rec_id,omitempty"`
	WorkspaceRecID string     `json:"workspace_rec_id"`
	WorkspaceName  string     `json:"workspace_name,omitempty"`
	Status         string     `json:"status"`
	Scope          string     `json:"scope,omitempty"`
	StartedAt      time.Time  `json:"started_at"`
	FinishedAt     *time.Time `json:"finished_at,omitempty"`
	Succeeded      int        `json:"succeeded"`
	Skipped        int        `json:"skipped"`
	Failed         int        `json:"failed"`
	Results        []Result   `json:"results,omitempty"`
	CurrentProject string     `json:"current_project,omitempty"`
	DoneCount      int        `json:"done_count"`
	TotalCount     int        `json:"total_count"`
	Phase          string     `json:"phase,omitempty"`
}

// Status is the API view for poll / dialog open.
type Status struct {
	Job
}

// Coordinator 按工作空间单飞：同一空间同时只跑一次自动构建。
type Coordinator struct {
	mu   sync.Mutex
	jobs map[string]*Job
}

// NewCoordinator constructs a per-workspace single-flight Coordinator.
func NewCoordinator() *Coordinator {
	return &Coordinator{jobs: map[string]*Job{}}
}

// Start begins an async build unless that workspace already has one running.
func (c *Coordinator) Start(ctx context.Context, workspaceRecID, workspaceName string, total int, meta Job, run func(ctx context.Context, job *Job)) error {
	workspaceRecID = strings.TrimSpace(workspaceRecID)
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
		return ErrBuildInProgress
	}
	now := time.Now().UTC()
	j := &Job{
		BuildRecID:     meta.BuildRecID,
		WorkspaceRecID: workspaceRecID,
		WorkspaceName:  workspaceName,
		Status:         JobStatusRunning,
		Scope:          meta.Scope,
		StartedAt:      now,
		TotalCount:     total,
		Phase:          "sync",
	}
	c.jobs[workspaceRecID] = j
	c.mu.Unlock()

	go func() {
		defer func() {
			if rec := recover(); rec != nil {
				log.Printf("build coordinator panic workspace=%s: %v\n%s", workspaceRecID, rec, debug.Stack())
			}
			c.mu.Lock()
			fin := time.Now().UTC()
			j.FinishedAt = &fin
			j.Status = JobStatusCompleted
			j.CurrentProject = ""
			j.Phase = ""
			c.mu.Unlock()
		}()
		run(ctx, j)
	}()
	return nil
}

// Status returns this workspace's job snapshot.
func (c *Coordinator) Status(queryWorkspace string) Status {
	queryWorkspace = strings.TrimSpace(queryWorkspace)
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

// SetPhase records sync / build / retry.
func (c *Coordinator) SetPhase(job *Job, phase string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if job == nil || job.Status != JobStatusRunning {
		return
	}
	job.Phase = phase
}

// SetProgress updates running job progress.
func (c *Coordinator) SetProgress(job *Job, current string, done, total int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if job == nil || job.Status != JobStatusRunning {
		return
	}
	job.CurrentProject = current
	job.DoneCount = done
	if total > 0 {
		job.TotalCount = total
	}
}

// AppendResult records one finished project.
func (c *Coordinator) AppendResult(job *Job, res Result, succeeded, skipped, failed int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if job == nil || job.Status != JobStatusRunning {
		return
	}
	job.Results = append(job.Results, res)
	job.Succeeded = succeeded
	job.Skipped = skipped
	job.Failed = failed
	job.DoneCount = len(job.Results)
}

// ReviseResult updates the latest result for a project, such as a retry.
func (c *Coordinator) ReviseResult(job *Job, projectName, status, message string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if job == nil {
		return
	}
	for i := len(job.Results) - 1; i >= 0; i-- {
		if job.Results[i].ProjectName != projectName {
			continue
		}
		job.Results[i].Status = status
		job.Results[i].Message = message
		return
	}
}

// FinishResult stores aggregate outcome on the running job.
func (c *Coordinator) FinishResult(job *Job, succeeded, skipped, failed int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if job == nil {
		return
	}
	job.Succeeded = succeeded
	job.Skipped = skipped
	job.Failed = failed
	job.DoneCount = len(job.Results)
}
