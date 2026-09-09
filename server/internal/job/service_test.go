package job_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"auto-pack/server/internal/access"
	"auto-pack/server/internal/iam"
	"auto-pack/server/internal/job"
	"auto-pack/server/internal/job/runner"
	"auto-pack/server/internal/migrate"
	"auto-pack/server/internal/project"
	"auto-pack/server/internal/store"
	"auto-pack/server/internal/user"
	"auto-pack/server/internal/workspace"
)

func setupJob(t *testing.T) (*job.Service, *iam.Guard, context.Context) {
	t.Helper()
	ctx := context.Background()
	st, err := store.Open(ctx, store.Config{Driver: "sqlite", Path: filepath.Join(t.TempDir(), "t.db")})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if _, err := migrate.Apply(ctx, st, "sqlite"); err != nil {
		t.Fatal(err)
	}
	if err := user.SeedDefaultAdmin(ctx, st); err != nil {
		t.Fatal(err)
	}
	if err := workspace.SeedDefaultWorkspace(ctx, st); err != nil {
		t.Fatal(err)
	}
	guard := iam.NewGuard(access.NewDirectory(st))
	prj := project.NewService(st, guard)
	eng := runner.NewEngine(st, prj, nil, t.TempDir(), "", 0)
	if err := eng.Start(ctx); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(eng.Stop)
	return job.NewService(st, guard, eng), guard, ctx
}

func TestSubmitRequiresAllow(t *testing.T) {
	svc, _, ctx := setupJob(t)
	_, err := svc.Submit(ctx, iam.Actor{}, job.SubmitRequest{
		WorkspaceRecID: access.DefaultWorkspaceRecID,
		JobType:        job.TypeBuild,
	})
	if !errors.Is(err, iam.ErrUnauthenticated) {
		t.Fatalf("want unauthenticated, got %v", err)
	}
}

func TestSubmitBuildAndFinish(t *testing.T) {
	svc, guard, ctx := setupJob(t)
	actor := iam.Actor{UserID: access.DefaultAdminRecID, LoginName: "admin"}
	if err := guard.Allow(ctx, actor, access.DefaultWorkspaceRecID, iam.ActionJobSubmit); err != nil {
		t.Fatal(err)
	}
	j, err := svc.Submit(ctx, actor, job.SubmitRequest{
		WorkspaceRecID: access.DefaultWorkspaceRecID,
		JobType:        job.TypeBuild,
		Payload:        job.Payload{Command: "echo ok"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if j.JobStatus != job.StatusQueued && j.JobStatus != job.StatusRunning && j.JobStatus != job.StatusSucceeded {
		t.Fatalf("status=%s", j.JobStatus)
	}
}

func TestListPagesWorkspaceJobs(t *testing.T) {
	svc, _, ctx := setupJob(t)
	actor := iam.Actor{UserID: access.DefaultAdminRecID, LoginName: "admin"}
	if _, err := svc.List(ctx, iam.Actor{}, job.ListQuery{WorkspaceID: access.DefaultWorkspaceRecID}); !errors.Is(err, iam.ErrUnauthenticated) {
		t.Fatalf("want unauthenticated, got %v", err)
	}
	page, err := svc.List(ctx, actor, job.ListQuery{WorkspaceID: access.DefaultWorkspaceRecID, Page: 1, PageSize: 20})
	if err != nil {
		t.Fatal(err)
	}
	if page.Items == nil || page.Live == nil || page.Page != 1 || page.PageSize != 20 {
		t.Fatalf("empty page: %+v", page)
	}
}

func TestViewerCannotSubmit(t *testing.T) {
	svc, _, ctx := setupJob(t)
	_, err := svc.Submit(ctx, iam.Actor{UserID: "missing"}, job.SubmitRequest{
		WorkspaceRecID: access.DefaultWorkspaceRecID,
		JobType:        job.TypeSyncSources,
	})
	if err == nil {
		t.Fatal("expected deny")
	}
}
