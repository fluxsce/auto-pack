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
	svc, ctx, actor, _ := setupEnvStore(t)
	return svc, ctx, actor
}

func setupEnvStore(t *testing.T) (*env.Service, context.Context, iam.Actor, *store.Store) {
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
	return env.NewService(st, guard, cipher), ctx, iam.Actor{UserID: access.DefaultAdminRecID, LoginName: "admin"}, st
}

func TestSaveKeepsSpecifiedRecIDOnInsert(t *testing.T) {
	svc, ctx, actor := setupEnv(t)
	id := "11111111-1111-4111-8111-111111111111"
	saved, err := svc.Save(ctx, actor, env.Env{
		RecID:          id,
		WorkspaceRecID: access.DefaultWorkspaceRecID,
		EnvKind:        "nodejs",
		EnvName:        "from-v1",
		IsEnabled:      1,
		Config:         map[string]any{"node_path": "/usr/bin/node"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if saved.RecID != id {
		t.Fatalf("rec_id=%s want %s", saved.RecID, id)
	}
	if saved.EnvKind != "node" {
		t.Fatalf("kind=%s", saved.EnvKind)
	}
	got, err := svc.Get(ctx, actor, id)
	if err != nil {
		t.Fatal(err)
	}
	if got.EnvName != "from-v1" {
		t.Fatalf("name=%s", got.EnvName)
	}
	if got.WorkspaceRecID != "" {
		t.Fatalf("workspace_rec_id=%q want empty", got.WorkspaceRecID)
	}
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

func TestListKindEnabledReturnsEveryMQ(t *testing.T) {
	svc, ctx, actor := setupEnv(t)
	saveMQ := func(name string, enabled int) {
		t.Helper()
		_, err := svc.Save(ctx, actor, env.Env{
			WorkspaceRecID: access.DefaultWorkspaceRecID,
			EnvKind:        "mq",
			EnvName:        name,
			IsEnabled:      enabled,
			Config: map[string]any{
				"endpoint":          "https://123.mns.cn-hangzhou.aliyuncs.com",
				"request_queue":     name + "-req",
				"result_queue":      name + "-res",
				"access_key_id":     "ak",
				"access_key_secret": "sk",
			},
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	saveMQ("mq-z", 1)
	saveMQ("mq-m", 1)
	if _, err := svc.Save(ctx, actor, env.Env{
		RecID:          "22222222-2222-4222-8222-222222222222",
		WorkspaceRecID: access.DefaultWorkspaceRecID,
		EnvKind:        "mq",
		EnvName:        "mq-a",
		IsEnabled:      0,
		Config: map[string]any{
			"endpoint":          "https://123.mns.cn-hangzhou.aliyuncs.com",
			"request_queue":     "mq-a-req",
			"result_queue":      "mq-a-res",
			"access_key_id":     "ak",
			"access_key_secret": "sk",
		},
	}); err != nil {
		t.Fatal(err)
	}
	one, _, err := svc.FindKind(ctx, "", "mq")
	if err != nil {
		t.Fatal(err)
	}
	if one.EnvName != "mq-m" {
		t.Fatalf("find=%s", one.EnvName)
	}
	list, cfgs, err := svc.ListKindEnabled(ctx, "mq")
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 || len(cfgs) != 2 {
		t.Fatalf("list=%d cfgs=%d", len(list), len(cfgs))
	}
	if list[0].EnvName != "mq-m" || list[1].EnvName != "mq-z" {
		t.Fatalf("%s %s", list[0].EnvName, list[1].EnvName)
	}
	if cfgs[1]["request_queue"] != "mq-z-req" {
		t.Fatalf("cfg=%v", cfgs[1])
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

func TestListIsGlobal(t *testing.T) {
	svc, ctx, actor, st := setupEnvStore(t)
	if _, err := svc.Save(ctx, actor, env.Env{
		WorkspaceRecID: access.DefaultWorkspaceRecID,
		EnvKind:        "maven",
		EnvName:        "mvn",
		Config:         map[string]any{"maven_home": "/opt/maven"},
	}); err != nil {
		t.Fatal(err)
	}
	spaces := workspace.NewService(st, iam.NewGuard(access.NewDirectory(st)), t.TempDir())
	other, err := spaces.Create(ctx, actor, "other", "其它空间")
	if err != nil {
		t.Fatal(err)
	}
	list, err := svc.List(ctx, actor, other.RecID)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].EnvName != "mvn" || list[0].WorkspaceRecID != "" {
		t.Fatalf("global list=%v", list)
	}
	e, cfg, err := svc.FindKind(ctx, other.RecID, "maven")
	if err != nil {
		t.Fatal(err)
	}
	if e.EnvName != "mvn" || cfg["maven_home"] != "/opt/maven" {
		t.Fatalf("global find name=%s cfg=%v", e.EnvName, cfg)
	}
}
