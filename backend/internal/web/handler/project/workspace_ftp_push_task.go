package project

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"auto-pack/backend/pkg/workspaceftppush"
)

const workspaceFtpPushLogTimeLayout = "2006-01-02 15:04:05"

type workspaceFtpPushTask struct {
	TaskID       string                   `json:"task_id"`
	ProjectRecID string                   `json:"project_rec_id"`
	Status       string                   `json:"status"`
	Message      string                   `json:"message,omitempty"`
	Path         string                   `json:"path,omitempty"`
	PushPath     string                   `json:"push_path,omitempty"`
	StartedAt    time.Time                `json:"started_at"`
	FinishedAt   *time.Time               `json:"finished_at,omitempty"`
	LogContent   string                   `json:"log_content,omitempty"`
	Result       *workspaceftppush.Result `json:"result,omitempty"`
}

type workspaceFtpPushCoordinator struct {
	mu        sync.Mutex
	byProject map[string]*workspaceFtpPushTask
	byTaskID  map[string]*workspaceFtpPushTask
}

func newWorkspaceFtpPushCoordinator() *workspaceFtpPushCoordinator {
	return &workspaceFtpPushCoordinator{
		byProject: make(map[string]*workspaceFtpPushTask),
		byTaskID:  make(map[string]*workspaceFtpPushTask),
	}
}

func (c *workspaceFtpPushCoordinator) start(
	taskID, projectRecID, relPath, pushPath string,
	run func(ctx context.Context, appendLog func(format string, args ...any)) (*workspaceftppush.Result, error),
) *workspaceFtpPushTask {
	now := time.Now().UTC()
	task := &workspaceFtpPushTask{
		TaskID:       strings.TrimSpace(taskID),
		ProjectRecID: strings.TrimSpace(projectRecID),
		Status:       "running",
		Path:         strings.TrimSpace(relPath),
		PushPath:     strings.TrimSpace(pushPath),
		StartedAt:    now,
	}
	c.mu.Lock()
	c.byProject[task.ProjectRecID] = task
	c.byTaskID[task.TaskID] = task
	c.mu.Unlock()

	go func() {
		ctx := context.Background()
		result, err := run(ctx, func(format string, args ...any) {
			c.appendLog(task.TaskID, format, args...)
		})
		c.finish(task.TaskID, result, err)
	}()
	return c.snapshot(task)
}

func (c *workspaceFtpPushCoordinator) activeTask(projectRecID string) *workspaceFtpPushTask {
	c.mu.Lock()
	defer c.mu.Unlock()
	task := c.byProject[strings.TrimSpace(projectRecID)]
	if task == nil || task.Status != "running" {
		return nil
	}
	return c.copyTask(task)
}

func (c *workspaceFtpPushCoordinator) status(projectRecID, taskID string) *workspaceFtpPushTask {
	c.mu.Lock()
	defer c.mu.Unlock()
	if id := strings.TrimSpace(taskID); id != "" {
		if task := c.byTaskID[id]; task != nil && task.ProjectRecID == strings.TrimSpace(projectRecID) {
			return c.copyTask(task)
		}
		return nil
	}
	if task := c.byProject[strings.TrimSpace(projectRecID)]; task != nil {
		return c.copyTask(task)
	}
	return nil
}

func (c *workspaceFtpPushCoordinator) appendLog(taskID, format string, args ...any) {
	c.mu.Lock()
	defer c.mu.Unlock()
	task := c.byTaskID[strings.TrimSpace(taskID)]
	if task == nil {
		return
	}
	line := "[" + time.Now().Format(workspaceFtpPushLogTimeLayout) + "] " + fmt.Sprintf(format, args...)
	if task.LogContent == "" {
		task.LogContent = line
	} else {
		task.LogContent += "\n" + line
	}
}

func (c *workspaceFtpPushCoordinator) finish(taskID string, result *workspaceftppush.Result, err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	task := c.byTaskID[strings.TrimSpace(taskID)]
	if task == nil {
		return
	}
	fin := time.Now().UTC()
	task.FinishedAt = &fin
	if err != nil {
		task.Status = "failed"
		task.Message = err.Error()
		if task.LogContent == "" {
			task.LogContent = "[" + time.Now().Format(workspaceFtpPushLogTimeLayout) + "] " + err.Error()
		} else {
			task.LogContent += "\n[" + time.Now().Format(workspaceFtpPushLogTimeLayout) + "] " + err.Error()
		}
		return
	}
	task.Status = "success"
	task.Message = "ok"
	task.Result = result
}

func (c *workspaceFtpPushCoordinator) snapshot(task *workspaceFtpPushTask) *workspaceFtpPushTask {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.copyTask(task)
}

func (c *workspaceFtpPushCoordinator) copyTask(task *workspaceFtpPushTask) *workspaceFtpPushTask {
	if task == nil {
		return nil
	}
	cp := *task
	if task.Result != nil {
		r := *task.Result
		cp.Result = &r
	}
	return &cp
}
