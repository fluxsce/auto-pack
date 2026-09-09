package audit_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"auto-pack/server/internal/access"
	"auto-pack/server/internal/audit"
	"auto-pack/server/internal/iam"
	"auto-pack/server/internal/migrate"
	"auto-pack/server/internal/store"
	"auto-pack/server/internal/user"
	"auto-pack/server/internal/workspace"
)

func setupAudit(t *testing.T) (*audit.Service, context.Context, iam.Actor) {
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
	actor := iam.Actor{UserID: access.DefaultAdminRecID, LoginName: "admin"}
	return audit.NewService(st, guard), ctx, actor
}

func TestRecordAndListScopedToWorkspace(t *testing.T) {
	svc, ctx, actor := setupAudit(t)
	ws := access.DefaultWorkspaceRecID
	if err := svc.Record(ctx, actor, ws, audit.ActionJobSubmit, "job", "j1", "submit build", true); err != nil {
		t.Fatal(err)
	}
	if err := svc.Record(ctx, actor, "other-ws", audit.ActionJobSubmit, "job", "j2", "other space", true); err != nil {
		t.Fatal(err)
	}
	if err := svc.Record(ctx, actor, "", audit.ActionWorkspaceCreate, "workspace", "w1", "global", true); err != nil {
		t.Fatal(err)
	}
	page, err := svc.List(ctx, actor, audit.ListQuery{WorkspaceID: ws})
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 1 || len(page.Items) != 1 {
		t.Fatalf("want 1 row in workspace, got total=%d items=%d", page.Total, len(page.Items))
	}
	if page.Items[0].TargetID != "j1" {
		t.Fatalf("leaked row: %+v", page.Items[0])
	}
	if page.Items[0].ActorLoginName != "admin" {
		t.Fatalf("actor login=%q", page.Items[0].ActorLoginName)
	}
}

func TestListFiltersAndPaging(t *testing.T) {
	svc, ctx, actor := setupAudit(t)
	ws := access.DefaultWorkspaceRecID
	if err := svc.Record(ctx, actor, ws, audit.ActionJobSubmit, "job", "a", "submit build", true); err != nil {
		t.Fatal(err)
	}
	if err := svc.Record(ctx, actor, ws, audit.ActionJobCancel, "job", "b", "cancel job", false); err != nil {
		t.Fatal(err)
	}
	if err := svc.Record(ctx, actor, ws, audit.ActionProjectSave, "project", "p1", "save demo", true); err != nil {
		t.Fatal(err)
	}

	fail, err := svc.List(ctx, actor, audit.ListQuery{WorkspaceID: ws, Result: "fail"})
	if err != nil {
		t.Fatal(err)
	}
	if fail.Total != 1 || fail.Items[0].ActionCode != audit.ActionJobCancel {
		t.Fatalf("fail filter: %+v", fail)
	}

	jobs, err := svc.List(ctx, actor, audit.ListQuery{WorkspaceID: ws, Action: audit.ActionJobSubmit})
	if err != nil {
		t.Fatal(err)
	}
	if jobs.Total != 1 || jobs.Items[0].Summary != "submit build" {
		t.Fatalf("action filter: %+v", jobs)
	}

	kw, err := svc.List(ctx, actor, audit.ListQuery{WorkspaceID: ws, Keyword: "demo"})
	if err != nil {
		t.Fatal(err)
	}
	if kw.Total != 1 || kw.Items[0].TargetID != "p1" {
		t.Fatalf("keyword filter: %+v", kw)
	}

	p1, err := svc.List(ctx, actor, audit.ListQuery{WorkspaceID: ws, Page: 1, PageSize: 2})
	if err != nil {
		t.Fatal(err)
	}
	if p1.Total != 3 || len(p1.Items) != 2 || p1.Page != 1 || p1.PageSize != 2 {
		t.Fatalf("page1: %+v", p1)
	}
	p2, err := svc.List(ctx, actor, audit.ListQuery{WorkspaceID: ws, Page: 2, PageSize: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(p2.Items) != 1 {
		t.Fatalf("page2 items=%d", len(p2.Items))
	}
}

func TestListRequiresAuditRead(t *testing.T) {
	svc, ctx, _ := setupAudit(t)
	_, err := svc.List(ctx, iam.Actor{}, audit.ListQuery{WorkspaceID: access.DefaultWorkspaceRecID})
	if !errors.Is(err, iam.ErrUnauthenticated) {
		t.Fatalf("want unauthenticated, got %v", err)
	}
}

func TestNoteRecordsFailure(t *testing.T) {
	svc, ctx, actor := setupAudit(t)
	svc.Note(ctx, actor, access.DefaultWorkspaceRecID, audit.ActionEnvDelete, "env", "e1", "delete ftp", errors.New("not found"))
	page, err := svc.List(ctx, actor, audit.ListQuery{WorkspaceID: access.DefaultWorkspaceRecID, Result: "fail"})
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 1 || page.Items[0].SuccessFlag != 0 {
		t.Fatalf("want failed note, got %+v", page)
	}
	if page.Items[0].Summary != "delete ftp: not found" {
		t.Fatalf("summary=%q", page.Items[0].Summary)
	}
}
