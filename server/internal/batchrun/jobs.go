// Package batchrun 供工作空间自动构建 / 发版后构建提交并等待单工程作业。
package batchrun

import (
	"context"
	"strings"
	"time"

	"auto-pack/server/internal/iam"
	"auto-pack/server/internal/job"
)

// PushSpec is the payload for one ftp_push job.
type PushSpec struct {
	EnvRecID   string
	PushPath   string
	BackupPath string
	Pusher     string
	TaskRef    string
	Detail     string
	Notes      string
}

// Outcome is one finished project job.
type Outcome struct {
	OK      bool
	Message string
	JobID   string
}

// StepFn receives the current running step name while waiting.
type StepFn func(step string)

// RunBuild submits a build job and waits until it finishes.
// skipSync 为 true 时作业不再 fetch/checkout（调用方刚做过空间预同步）。
func RunBuild(ctx context.Context, jobs *job.Service, actor iam.Actor, workspaceID, projectID string, skipSync bool, onStep StepFn) Outcome {
	if jobs == nil {
		return Outcome{Message: "job service unavailable"}
	}
	created, err := jobs.Submit(ctx, actor, job.SubmitRequest{
		WorkspaceRecID: workspaceID,
		ProjectRecID:   projectID,
		JobType:        job.TypeBuild,
		Payload:        job.Payload{SkipSync: skipSync, Origin: job.OriginAutoBuild},
	})
	if err != nil {
		return Outcome{Message: err.Error()}
	}
	return wait(ctx, jobs, created.RecID, onStep)
}

// RunPush submits an ftp_push job and waits until it finishes.
func RunPush(ctx context.Context, jobs *job.Service, actor iam.Actor, workspaceID, projectID string, spec PushSpec, onStep StepFn) Outcome {
	if jobs == nil {
		return Outcome{Message: "job service unavailable"}
	}
	notes := strings.TrimSpace(spec.Notes)
	if notes == "" {
		notes = strings.TrimSpace(strings.TrimSpace(spec.TaskRef) + " | " + strings.TrimSpace(spec.Detail))
		notes = strings.Trim(notes, " |")
	}
	created, err := jobs.Submit(ctx, actor, job.SubmitRequest{
		WorkspaceRecID: workspaceID,
		ProjectRecID:   projectID,
		JobType:        job.TypeFTPPush,
		Payload: job.Payload{
			EnvRecID:   spec.EnvRecID,
			RemotePath: spec.PushPath,
			BackupPath: spec.BackupPath,
			Pusher:     spec.Pusher,
			TaskRef:    spec.TaskRef,
			Detail:     spec.Detail,
			Notes:      notes,
		},
	})
	if err != nil {
		return Outcome{Message: err.Error()}
	}
	return wait(ctx, jobs, created.RecID, onStep)
}

func wait(ctx context.Context, jobs *job.Service, id string, onStep StepFn) Outcome {
	if ctx == nil {
		ctx = context.Background()
	}
	last := ""
	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	for {
		j, steps, err := peek(ctx, jobs, id)
		if err != nil {
			return Outcome{JobID: id, Message: err.Error()}
		}
		if name := job.RunningStepName(steps); name != "" && name != last {
			last = name
			if onStep != nil {
				onStep(name)
			}
		}
		if job.Terminal(j.JobStatus) {
			ok := j.JobStatus == job.StatusSucceeded
			msg := strings.TrimSpace(j.ErrorText)
			if msg == "" {
				msg = j.JobStatus
			}
			return Outcome{OK: ok, Message: msg, JobID: id}
		}
		select {
		case <-ctx.Done():
			return Outcome{JobID: id, Message: ctx.Err().Error()}
		case <-tick.C:
		}
	}
}

func peek(ctx context.Context, jobs *job.Service, id string) (job.Job, []job.Step, error) {
	return jobs.Peek(ctx, id)
}
