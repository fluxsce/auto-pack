package job

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"auto-pack/server/internal/migrate"
	"auto-pack/server/internal/store"
)

func setupJobDB(t *testing.T) (*store.Store, context.Context) {
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
	return st, ctx
}

func putTestJob(t *testing.T, ctx context.Context, st *store.Store, j Job) {
	t.Helper()
	if j.PayloadJSON == "" {
		j.PayloadJSON = "{}"
	}
	if j.CreatedBy == "" {
		j.CreatedBy = "u1"
	}
	if err := insertJob(ctx, st, j); err != nil {
		t.Fatal(err)
	}
	if j.CreatedAt == "" && j.FinishedAt == "" && j.StartedAt == "" {
		return
	}
	q := st.Rebind(`UPDATE AP_JOB SET started_at=?, finished_at=?, created_at=? WHERE rec_id=?`)
	created := j.CreatedAt
	if created == "" {
		created = NowUTC()
	}
	if _, err := st.SQL().ExecContext(ctx, q, nullStr(j.StartedAt), nullStr(j.FinishedAt), created, j.RecID); err != nil {
		t.Fatal(err)
	}
}

func TestListJobsPage(t *testing.T) {
	st, ctx := setupJobDB(t)
	ws := "ws-1"
	putTestJob(t, ctx, st, Job{RecID: "j1", WorkspaceRecID: ws, ProjectRecID: "p1", JobType: TypeBuild, JobStatus: StatusSucceeded, CreatedAt: "2026-09-01 10:00:00"})
	putTestJob(t, ctx, st, Job{RecID: "j2", WorkspaceRecID: ws, ProjectRecID: "p1", JobType: TypeRelease, JobStatus: StatusFailed, CreatedAt: "2026-09-02 10:00:00"})
	putTestJob(t, ctx, st, Job{RecID: "j3", WorkspaceRecID: ws, ProjectRecID: "p2", JobType: TypeBuild, JobStatus: StatusRunning, CreatedAt: "2026-09-03 10:00:00"})
	putTestJob(t, ctx, st, Job{RecID: "j4", WorkspaceRecID: "other", ProjectRecID: "p1", JobType: TypeBuild, JobStatus: StatusSucceeded, CreatedAt: "2026-09-04 10:00:00"})

	page, err := listJobsPage(ctx, st, ListQuery{WorkspaceID: ws, Page: 1, PageSize: 2})
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 3 || page.Page != 1 || page.PageSize != 2 || len(page.Items) != 2 {
		t.Fatalf("page1: %+v items=%d", page, len(page.Items))
	}
	if page.Items[0].RecID != "j3" || page.Items[1].RecID != "j2" {
		t.Fatalf("order: %s %s", page.Items[0].RecID, page.Items[1].RecID)
	}
	if page.Stats.Running != 1 || page.Stats.Succeeded != 1 || page.Stats.Failed != 1 {
		t.Fatalf("stats: %+v", page.Stats)
	}
	if len(page.Live) != 1 || page.Live[0].RecID != "j3" {
		t.Fatalf("live: %+v", page.Live)
	}

	page2, err := listJobsPage(ctx, st, ListQuery{WorkspaceID: ws, Page: 2, PageSize: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(page2.Items) != 1 || page2.Items[0].RecID != "j1" {
		t.Fatalf("page2: %+v", page2.Items)
	}

	filtered, err := listJobsPage(ctx, st, ListQuery{WorkspaceID: ws, ProjectID: "p1", JobType: TypeBuild, Page: 1, PageSize: 20})
	if err != nil {
		t.Fatal(err)
	}
	if filtered.Total != 1 || filtered.Items[0].RecID != "j1" {
		t.Fatalf("filter: %+v", filtered)
	}
}

func TestExpiredFinishedAndDelete(t *testing.T) {
	st, ctx := setupJobDB(t)
	old := time.Now().UTC().AddDate(0, 0, -10).Format(TimeLayout)
	recent := time.Now().UTC().AddDate(0, 0, -1).Format(TimeLayout)
	putTestJob(t, ctx, st, Job{RecID: "old", WorkspaceRecID: "ws", ProjectRecID: "p1", JobType: TypeBuild, JobStatus: StatusSucceeded, FinishedAt: old, CreatedAt: old})
	putTestJob(t, ctx, st, Job{RecID: "new", WorkspaceRecID: "ws", ProjectRecID: "p1", JobType: TypeBuild, JobStatus: StatusFailed, FinishedAt: recent, CreatedAt: recent})
	putTestJob(t, ctx, st, Job{RecID: "run", WorkspaceRecID: "ws", ProjectRecID: "p1", JobType: TypeBuild, JobStatus: StatusRunning, CreatedAt: old})
	if err := insertStep(ctx, st, newStep("old", "build", "构建", 1)); err != nil {
		t.Fatal(err)
	}

	cutoff := time.Now().UTC().AddDate(0, 0, -7).Format(TimeLayout)
	expired, err := ExpiredFinished(ctx, st, "p1", "ws", cutoff, 200)
	if err != nil {
		t.Fatal(err)
	}
	if len(expired) != 1 || expired[0].RecID != "old" {
		t.Fatalf("expired=%+v", expired)
	}

	if err := DeleteFinished(ctx, st, "run"); err != nil {
		t.Fatal(err)
	}
	if _, err := JobByID(ctx, st, "run"); err != nil {
		t.Fatal("running job must stay")
	}

	if err := DeleteFinished(ctx, st, "old"); err != nil {
		t.Fatal(err)
	}
	if _, err := JobByID(ctx, st, "old"); err == nil {
		t.Fatal("old job should be gone")
	}
	steps, err := StepsByJob(ctx, st, "old")
	if err != nil {
		t.Fatal(err)
	}
	if len(steps) != 0 {
		t.Fatalf("steps left: %d", len(steps))
	}
	if _, err := JobByID(ctx, st, "new"); err != nil {
		t.Fatal(err)
	}
}

func TestExpiredFinishedNoProject(t *testing.T) {
	st, ctx := setupJobDB(t)
	old := time.Now().UTC().AddDate(0, 0, -10).Format(TimeLayout)
	putTestJob(t, ctx, st, Job{RecID: "bare", WorkspaceRecID: "ws", JobType: TypeSyncSources, JobStatus: StatusCanceled, FinishedAt: old, CreatedAt: old})
	putTestJob(t, ctx, st, Job{RecID: "owned", WorkspaceRecID: "ws", ProjectRecID: "p1", JobType: TypeBuild, JobStatus: StatusSucceeded, FinishedAt: old, CreatedAt: old})

	cutoff := time.Now().UTC().AddDate(0, 0, -7).Format(TimeLayout)
	expired, err := ExpiredFinished(ctx, st, "", "ws", cutoff, 200)
	if err != nil {
		t.Fatal(err)
	}
	if len(expired) != 1 || expired[0].RecID != "bare" {
		t.Fatalf("expired=%+v", expired)
	}
}
