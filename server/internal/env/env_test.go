package env_test

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"

	"auto-pack/server/internal/access"
	"auto-pack/server/internal/crypt"
	"auto-pack/server/internal/env"
	"auto-pack/server/internal/iam"
	"auto-pack/server/internal/migrate"
	"auto-pack/server/internal/store"
	"auto-pack/server/internal/user"
	"auto-pack/server/internal/workspace"
)

func setupEnv(t *testing.T) (*env.Service, context.Context, iam.Actor) {
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
	cipher, err := crypt.NewFromPassphrase("test-key")
	if err != nil {
		t.Fatal(err)
	}
	guard := iam.NewGuard(access.NewDirectory(st))
	return env.NewService(st, guard, cipher), ctx, iam.Actor{UserID: access.DefaultAdminRecID, LoginName: "admin"}
}

func TestSaveGetKeepsSecretOnEmptyUpdate(t *testing.T) {
	svc, ctx, actor := setupEnv(t)
	saved, err := svc.Save(ctx, actor, env.Env{
		WorkspaceRecID: access.DefaultWorkspaceRecID,
		EnvKind:        "ftp",
		EnvName:        "prod-ftp",
		Config: map[string]any{
			"host":     "ftp.local",
			"username": "u",
			"password": "p1",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	got, err := svc.Get(ctx, actor, saved.RecID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Config["password"] != nil {
		t.Fatal("http get must not return password")
	}
	if len(got.SecretsSet) == 0 {
		t.Fatal("secrets_set should mark password")
	}
	if _, err := svc.Save(ctx, actor, env.Env{
		RecID:          saved.RecID,
		WorkspaceRecID: access.DefaultWorkspaceRecID,
		EnvKind:        "ftp",
		EnvName:        "prod-ftp",
		IsEnabled:      1,
		Config:         map[string]any{"host": "ftp.local", "username": "u2", "password": ""},
	}); err != nil {
		t.Fatal(err)
	}
	raw, err := svc.DecryptRaw(ctx, saved.RecID)
	if err != nil {
		t.Fatal(err)
	}
	if raw["password"] != "p1" {
		t.Fatalf("password=%v", raw["password"])
	}
	if raw["username"] != "u2" {
		t.Fatalf("username=%v", raw["username"])
	}
}

func TestSaveRejectsUnknownKind(t *testing.T) {
	svc, ctx, actor := setupEnv(t)
	_, err := svc.Save(ctx, actor, env.Env{
		WorkspaceRecID: access.DefaultWorkspaceRecID,
		EnvKind:        "oracle",
		EnvName:        "db",
		Config:         map[string]any{"host": "x"},
	})
	if err == nil {
		t.Fatal("unknown kind")
	}
}

func TestFindKindAcceptsNodeAlias(t *testing.T) {
	svc, ctx, actor := setupEnv(t)
	if _, err := svc.Save(ctx, actor, env.Env{
		WorkspaceRecID: access.DefaultWorkspaceRecID,
		EnvKind:        "nodejs",
		EnvName:        "node-lts",
		Config:         map[string]any{"node_path": "/usr/bin/node"},
	}); err != nil {
		t.Fatal(err)
	}
	e, cfg, err := svc.FindKind(ctx, access.DefaultWorkspaceRecID, "node")
	if err != nil {
		t.Fatal(err)
	}
	if e.EnvKind != "node" {
		t.Fatalf("kind=%s", e.EnvKind)
	}
	if cfg["node_path"] != "/usr/bin/node" {
		t.Fatalf("cfg=%v", cfg)
	}
}

func TestDelete(t *testing.T) {
	svc, ctx, actor := setupEnv(t)
	saved, err := svc.Save(ctx, actor, env.Env{
		WorkspaceRecID: access.DefaultWorkspaceRecID,
		EnvKind:        "jdk",
		EnvName:        "jdk21",
		Config:         map[string]any{"java_home": "/opt/jdk"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.Delete(ctx, actor, saved.RecID); err != nil {
		t.Fatal(err)
	}
	_, err = svc.Get(ctx, actor, saved.RecID)
	if !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("want missing, got %v", err)
	}
}

func TestProbeSavedRowUsesStoredConfig(t *testing.T) {
	svc, ctx, actor := setupEnv(t)
	home := t.TempDir()
	saved, err := svc.Save(ctx, actor, env.Env{
		WorkspaceRecID: access.DefaultWorkspaceRecID,
		EnvKind:        "jdk",
		EnvName:        "jdk21",
		Config:         map[string]any{"java_home": home},
	})
	if err != nil {
		t.Fatal(err)
	}
	res, err := svc.Probe(ctx, actor, env.ProbeIn{
		WorkspaceRecID: access.DefaultWorkspaceRecID,
		RecID:          saved.RecID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !res.OK {
		t.Fatalf("list-row probe must use stored config, got %s", res.Message)
	}
}

func TestProbeUnknownKind(t *testing.T) {
	svc, ctx, actor := setupEnv(t)
	_, err := svc.Probe(ctx, actor, env.ProbeIn{
		WorkspaceRecID: access.DefaultWorkspaceRecID,
		EnvKind:        "oracle",
		Config:         map[string]any{},
	})
	if err == nil {
		t.Fatal("unknown kind")
	}
}

func TestListKinds(t *testing.T) {
	svc, _, _ := setupEnv(t)
	if len(svc.ListKinds()) < 8 {
		t.Fatalf("kinds=%d", len(svc.ListKinds()))
	}
}
