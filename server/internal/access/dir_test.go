package access_test

import (
	"context"
	"path/filepath"
	"testing"

	"auto-pack/server/internal/access"
	"auto-pack/server/internal/iam"
	"auto-pack/server/internal/migrate"
	"auto-pack/server/internal/store"
	"auto-pack/server/internal/user"
	"auto-pack/server/internal/workspace"
)

func TestFindMemberUsesBestRole(t *testing.T) {
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
	dir := access.NewDirectory(st)
	m, err := dir.FindMember(ctx, "", access.DefaultAdminRecID)
	if err != nil {
		t.Fatal(err)
	}
	if m.Role != iam.RoleOwner {
		t.Fatalf("role=%q", m.Role)
	}
	other := access.NewID()
	q := st.Rebind(`INSERT INTO AP_WORKSPACE_MEMBER (rec_id, workspace_rec_id, user_rec_id, member_role)
		VALUES (?, ?, ?, ?)`)
	if _, err := st.SQL().ExecContext(ctx, q, access.NewID(), access.DefaultWorkspaceRecID, other, iam.RoleViewer); err != nil {
		t.Fatal(err)
	}
	if _, err := st.SQL().ExecContext(ctx, q, access.NewID(), access.GlobalMemberWorkspaceID, other, iam.RoleOperator); err != nil {
		t.Fatal(err)
	}
	got, err := dir.FindMember(ctx, access.DefaultWorkspaceRecID, other)
	if err != nil {
		t.Fatal(err)
	}
	if got.Role != iam.RoleOperator {
		t.Fatalf("want operator, got %q", got.Role)
	}
}
