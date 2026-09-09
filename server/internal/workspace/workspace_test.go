package workspace_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"auto-pack/server/internal/access"
	"auto-pack/server/internal/iam"
	"auto-pack/server/internal/migrate"
	"auto-pack/server/internal/store"
	"auto-pack/server/internal/user"
	"auto-pack/server/internal/workspace"
)

func openWS(t *testing.T) (*workspace.Service, *store.Store, context.Context, iam.Actor, *user.Service) {
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
	users := user.NewService(st, guard, access.NewDirectory(st))
	return workspace.NewService(st, guard, t.TempDir()), st, ctx, iam.Actor{UserID: access.DefaultAdminRecID, LoginName: "admin"}, users
}

func setupWS(t *testing.T) (*workspace.Service, context.Context, iam.Actor, *user.Service) {
	t.Helper()
	svc, _, ctx, actor, users := openWS(t)
	return svc, ctx, actor, users
}

func insertProject(t *testing.T, st *store.Store, ctx context.Context, wsID, name string) {
	t.Helper()
	q := st.Rebind(`INSERT INTO AP_PROJECT (rec_id, workspace_rec_id, project_name, project_type, recipe_json, is_enabled)
		VALUES (?, ?, ?, ?, ?, 1)`)
	if _, err := st.SQL().ExecContext(ctx, q, access.NewID(), wsID, name, "maven", `{"command":"mvn package"}`); err != nil {
		t.Fatal(err)
	}
}

func TestRefuseDemoteLastOwner(t *testing.T) {
	svc, ctx, actor, _ := setupWS(t)
	err := svc.AddMember(ctx, actor, access.DefaultWorkspaceRecID, access.DefaultAdminRecID, iam.RoleOperator)
	if !errors.Is(err, workspace.ErrLastOwner) {
		t.Fatalf("want last owner, got %v", err)
	}
}

func TestRefuseRemoveLastOwner(t *testing.T) {
	svc, ctx, actor, _ := setupWS(t)
	err := svc.RemoveMember(ctx, actor, access.DefaultWorkspaceRecID, access.DefaultAdminRecID)
	if !errors.Is(err, workspace.ErrLastOwner) {
		t.Fatalf("want last owner, got %v", err)
	}
}

func TestDemoteOwnerAfterAddingAnother(t *testing.T) {
	svc, ctx, actor, users := setupWS(t)
	other, err := users.Create(ctx, actor, access.DefaultWorkspaceRecID, "ops", "运维", "123456", true)
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.AddMember(ctx, actor, access.DefaultWorkspaceRecID, other.RecID, iam.RoleOwner); err != nil {
		t.Fatal(err)
	}
	if err := svc.AddMember(ctx, actor, access.DefaultWorkspaceRecID, access.DefaultAdminRecID, iam.RoleOperator); err != nil {
		t.Fatal(err)
	}
}

func TestCloneCopiesProjects(t *testing.T) {
	svc, st, ctx, actor, _ := openWS(t)
	src, err := svc.Create(ctx, actor, "src", "源空间")
	if err != nil {
		t.Fatal(err)
	}
	insertProject(t, st, ctx, src.RecID, "demo")
	out, err := svc.Clone(ctx, actor, workspace.CloneRequest{
		SourceRecID:   src.RecID,
		WorkspaceCode: "src-copy",
		WorkspaceName: "源空间副本",
	})
	if err != nil {
		t.Fatal(err)
	}
	if out.ProjectsCloned != 1 || out.ProjectCount != 1 {
		t.Fatalf("cloned=%d count=%d", out.ProjectsCloned, out.ProjectCount)
	}
	if out.RecID == src.RecID || out.WorkspaceCode != "src-copy" {
		t.Fatalf("unexpected clone %+v", out)
	}
}

func TestDeleteWorkspaceAndProjects(t *testing.T) {
	svc, st, ctx, actor, _ := openWS(t)
	ws, err := svc.Create(ctx, actor, "tmp", "临时")
	if err != nil {
		t.Fatal(err)
	}
	insertProject(t, st, ctx, ws.RecID, "demo")
	if err := svc.Delete(ctx, actor, ws.RecID); err != nil {
		t.Fatal(err)
	}
	list, err := svc.ListForActor(ctx, actor)
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range list {
		if item.RecID == ws.RecID {
			t.Fatal("deleted workspace still listed")
		}
	}
}

func TestRefuseDeleteDefaultWorkspace(t *testing.T) {
	svc, ctx, actor, _ := setupWS(t)
	err := svc.Delete(ctx, actor, access.DefaultWorkspaceRecID)
	if !errors.Is(err, workspace.ErrDefaultWorkspace) {
		t.Fatalf("want default, got %v", err)
	}
}

func TestRefuseDeleteWhenJobRunning(t *testing.T) {
	svc, st, ctx, actor, _ := openWS(t)
	ws, err := svc.Create(ctx, actor, "busy", "忙碌")
	if err != nil {
		t.Fatal(err)
	}
	q := st.Rebind(`INSERT INTO AP_JOB (rec_id, workspace_rec_id, job_type, job_status, payload_json, created_by)
		VALUES (?, ?, 'build', 'running', '{}', ?)`)
	if _, err := st.SQL().ExecContext(ctx, q, access.NewID(), ws.RecID, actor.UserID); err != nil {
		t.Fatal(err)
	}
	err = svc.Delete(ctx, actor, ws.RecID)
	if !errors.Is(err, workspace.ErrWorkspaceBusy) {
		t.Fatalf("want busy, got %v", err)
	}
}

func TestCreateRejectsUnsafeCode(t *testing.T) {
	svc, ctx, actor, _ := setupWS(t)
	_, err := svc.Create(ctx, actor, "bad/name", "演示")
	if !errors.Is(err, workspace.ErrInvalidCode) {
		t.Fatalf("want code, got %v", err)
	}
	_, err = svc.Create(ctx, actor, "V9.0.0-P09", "版本空间")
	if err != nil {
		t.Fatalf("version code: %v", err)
	}
	_, err = svc.Create(ctx, actor, "ok-code", "bad/name")
	if !errors.Is(err, workspace.ErrInvalidName) {
		t.Fatalf("want name, got %v", err)
	}
}

func TestCreateAndDeleteWorkspaceDir(t *testing.T) {
	svc, ctx, actor, _ := setupWS(t)
	ws, err := svc.Create(ctx, actor, "disk-ws", "磁盘空间")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(ws.RootDir); err != nil {
		t.Fatal(err)
	}
	if err := svc.Delete(ctx, actor, ws.RecID); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(ws.RootDir); !os.IsNotExist(err) {
		t.Fatalf("dir still exists: %v", err)
	}
}

func TestViewerCannotDeleteWorkspace(t *testing.T) {
	svc, ctx, actor, users := setupWS(t)
	ws, err := svc.Create(ctx, actor, "ops", "运维空间")
	if err != nil {
		t.Fatal(err)
	}
	other, err := users.Create(ctx, actor, ws.RecID, "view", "只读", "123456", true)
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.AddMember(ctx, actor, ws.RecID, other.RecID, iam.RoleViewer); err != nil {
		t.Fatal(err)
	}
	err = svc.Delete(ctx, iam.Actor{UserID: other.RecID, LoginName: "view"}, ws.RecID)
	if !errors.Is(err, iam.ErrForbidden) {
		t.Fatalf("want forbidden, got %v", err)
	}
}

func TestClearCheckoutKeepsProjectDir(t *testing.T) {
	svc, ctx, actor, users := setupWS(t)
	list, err := svc.ListForActor(ctx, actor)
	if err != nil || len(list) == 0 {
		t.Fatalf("list: %v", err)
	}
	ws := list[0]
	proj := filepath.Join(ws.RootDir, "demo-app")
	if err := os.MkdirAll(filepath.Join(proj, "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(proj, "pom.xml"), []byte("<p/>"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := svc.ClearCheckout(ctx, actor, ws.RecID, "demo-app"); err != nil {
		t.Fatal(err)
	}
	st, err := os.Stat(proj)
	if err != nil || !st.IsDir() {
		t.Fatalf("project dir: %v", err)
	}
	items, err := os.ReadDir(proj)
	if err != nil || len(items) != 0 {
		t.Fatalf("want empty, got %v %v", items, err)
	}

	viewer, err := users.Create(ctx, actor, ws.RecID, "look", "只读", "123456", true)
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.AddMember(ctx, actor, ws.RecID, viewer.RecID, iam.RoleViewer); err != nil {
		t.Fatal(err)
	}
	err = svc.ClearCheckout(ctx, iam.Actor{UserID: viewer.RecID, LoginName: "look"}, ws.RecID, "demo-app")
	if !errors.Is(err, iam.ErrForbidden) {
		t.Fatalf("viewer: %v", err)
	}
}
