package job

import (
	"context"
	"errors"
	"fmt"

	"auto-pack/server/internal/access"
	"auto-pack/server/internal/iam"
	"auto-pack/server/internal/store"
)

// Service is the L3 job use-case layer. Writes go through iam.Allow.
type Service struct {
	st    *store.Store
	guard *iam.Guard
	run   Runner
}

// NewService returns a job Service. run 可为 nil（只入队不执行）。
func NewService(st *store.Store, guard *iam.Guard, run Runner) *Service {
	return &Service{st: st, guard: guard, run: run}
}

// SubmitRequest is the HTTP/cron input to create a job.
type SubmitRequest struct {
	WorkspaceRecID string  `json:"workspace_rec_id"`
	ProjectRecID   string  `json:"project_rec_id"`
	JobType        string  `json:"job_type"`
	Payload        Payload `json:"payload"`
}

// Submit creates a queued job after iam.Allow(job.submit).
func (s *Service) Submit(ctx context.Context, actor iam.Actor, req SubmitRequest) (Job, error) {
	if err := s.guard.Allow(ctx, actor, req.WorkspaceRecID, iam.ActionJobSubmit); err != nil {
		return Job{}, err
	}
	if req.JobType == "" {
		return Job{}, errors.New("job_type is required")
	}
	j := Job{
		RecID:          access.NewID(),
		WorkspaceRecID: req.WorkspaceRecID,
		ProjectRecID:   req.ProjectRecID,
		JobType:        req.JobType,
		JobStatus:      StatusQueued,
		PayloadJSON:    encodePayload(req.Payload),
		CreatedBy:      actor.UserID,
	}
	steps := stepsFor(req.JobType, req.Payload)
	if len(steps) == 0 {
		return Job{}, fmt.Errorf("unknown job_type %s", req.JobType)
	}
	err := s.st.Tx(ctx, func(tx *store.Store) error {
		if err := insertJob(ctx, tx, j); err != nil {
			return err
		}
		for i := range steps {
			steps[i].JobRecID = j.RecID
			if err := insertStep(ctx, tx, steps[i]); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return Job{}, err
	}
	if s.run != nil {
		s.run.Enqueue(j.RecID)
	}
	return j, nil
}

// SubmitSystem enqueues a job without an HTTP actor (cron). Still records created_by=system.
func (s *Service) SubmitSystem(ctx context.Context, req SubmitRequest) (Job, error) {
	return s.Submit(ctx, iam.Actor{UserID: access.DefaultAdminRecID, LoginName: "system"}, req)
}

// List returns a page of jobs the actor can read, plus live rows and filter-wide counts.
func (s *Service) List(ctx context.Context, actor iam.Actor, q ListQuery) (Page, error) {
	if err := s.guard.CanRead(ctx, actor, q.WorkspaceID); err != nil {
		return Page{}, err
	}
	return listJobsPage(ctx, s.st, q)
}

// Get returns a job and its steps.
func (s *Service) Get(ctx context.Context, actor iam.Actor, id string) (Job, []Step, error) {
	j, err := JobByID(ctx, s.st, id)
	if err != nil {
		return Job{}, nil, err
	}
	if err := s.guard.CanRead(ctx, actor, j.WorkspaceRecID); err != nil {
		return Job{}, nil, err
	}
	steps, err := StepsByJob(ctx, s.st, id)
	return j, steps, err
}

// Log returns disk log after a read check. offset is a byte position; 0 reads from the start.
func (s *Service) Log(ctx context.Context, actor iam.Actor, id string, offset int64) (string, int64, int64, error) {
	j, err := JobByID(ctx, s.st, id)
	if err != nil {
		return "", 0, 0, err
	}
	if err := s.guard.CanRead(ctx, actor, j.WorkspaceRecID); err != nil {
		return "", 0, 0, err
	}
	if s.run == nil {
		return "", 0, 0, nil
	}
	return s.run.ReadLog(id, offset)
}

// Cancel marks a queued/running job canceled after iam.Allow(job.cancel).
func (s *Service) Cancel(ctx context.Context, actor iam.Actor, id string) error {
	j, err := JobByID(ctx, s.st, id)
	if err != nil {
		return err
	}
	if err := s.guard.Allow(ctx, actor, j.WorkspaceRecID, iam.ActionJobCancel); err != nil {
		return err
	}
	if j.JobStatus != StatusQueued && j.JobStatus != StatusRunning {
		return fmt.Errorf("job is %s", j.JobStatus)
	}
	return SetJobStatus(ctx, s.st, id, StatusCanceled, "canceled by user", false, true)
}

func stepsFor(jobType string, p Payload) []Step {
	id := ""
	switch jobType {
	case TypeBuild:
		return []Step{newStep(id, "sync", "同步源码", 1), newStep(id, "build", "构建", 2), newStep(id, "post", "构建后", 3)}
	case TypeRelease:
		steps := []Step{newStep(id, "sync", "同步源码", 1), newStep(id, "version_txt", "写 Version.txt", 2)}
		if p.UpdatePOM {
			steps = append(steps, newStep(id, "pom", "更新 POM", 3))
		}
		steps = append(steps, newStep(id, "tag", "打 Git Tag", len(steps)+1))
		return steps
	case TypeFTPPush:
		return []Step{newStep(id, "ftp", "FTP 推送", 1)}
	case TypePOMVersion:
		return []Step{newStep(id, "sync", "同步源码", 1), newStep(id, "pom", "更新 POM", 2)}
	case TypeBranchClone:
		return []Step{newStep(id, "clone", "克隆分支", 1)}
	case TypeSyncSources:
		return []Step{newStep(id, "sync", "同步源码", 1)}
	default:
		return nil
	}
}

// MustAllow is used by tests to prove write paths call the guard.
func (s *Service) MustAllow(ctx context.Context, actor iam.Actor, workspaceID string, action iam.Action) error {
	return s.guard.Allow(ctx, actor, workspaceID, action)
}
