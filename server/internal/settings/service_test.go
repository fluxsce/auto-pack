package settings_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"auto-pack/server/internal/access"
	"auto-pack/server/internal/iam"
	"auto-pack/server/internal/migrate"
	"auto-pack/server/internal/settings"
	"auto-pack/server/internal/store"
	"auto-pack/server/internal/user"
	"auto-pack/server/internal/workspace"
)

func setupSettings(t *testing.T) (*settings.Service, context.Context) {
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
	return settings.NewService(st, iam.NewGuard(access.NewDirectory(st))), ctx
}

func TestPropsRoundTrip(t *testing.T) {
	svc, ctx := setupSettings(t)
	actor := iam.Actor{UserID: access.DefaultAdminRecID, LoginName: "admin"}
	got, err := svc.GetProps(ctx, actor, access.DefaultWorkspaceRecID)
	if err != nil {
		t.Fatal(err)
	}
	if got.JobConcurrency != 6 || got.WebTimeoutSec != 60 {
		t.Fatalf("%+v", got)
	}
	saved, err := svc.SaveProps(ctx, actor, access.DefaultWorkspaceRecID, settings.Props{
		JobConcurrency:   4,
		LogRetentionDays: 14,
		WebTimeoutSec:    120,
	})
	if err != nil {
		t.Fatal(err)
	}
	if saved.JobConcurrency != 4 || saved.LogRetentionDays != 14 || saved.WebTimeoutSec != 120 {
		t.Fatalf("%+v", saved)
	}
	again, err := svc.GetProps(ctx, actor, access.DefaultWorkspaceRecID)
	if err != nil {
		t.Fatal(err)
	}
	if again != saved {
		t.Fatalf("got %+v want %+v", again, saved)
	}
}

func TestPropsClampAndAuth(t *testing.T) {
	svc, ctx := setupSettings(t)
	actor := iam.Actor{UserID: access.DefaultAdminRecID}
	p, err := svc.SaveProps(ctx, actor, access.DefaultWorkspaceRecID, settings.Props{
		JobConcurrency:   99,
		LogRetentionDays: -3,
		WebTimeoutSec:    1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if p.JobConcurrency != settings.MaxJobConcurrency || p.LogRetentionDays != 0 || p.WebTimeoutSec != settings.MinWebTimeoutSec {
		t.Fatalf("%+v", p)
	}
	if _, err := svc.SaveProps(ctx, iam.Actor{}, access.DefaultWorkspaceRecID, p); !errors.Is(err, iam.ErrUnauthenticated) {
		t.Fatalf("got %v", err)
	}
}

func TestVarsSaveListDelete(t *testing.T) {
	svc, ctx := setupSettings(t)
	actor := iam.Actor{UserID: access.DefaultAdminRecID}
	if _, err := svc.SaveVar(ctx, actor, access.DefaultWorkspaceRecID, settings.Var{Key: "PATH", Value: "x"}); !errors.Is(err, settings.ErrReserved) {
		t.Fatalf("reserved: %v", err)
	}
	if _, err := svc.SaveVar(ctx, actor, access.DefaultWorkspaceRecID, settings.Var{Key: "1bad", Value: "x"}); !errors.Is(err, settings.ErrBadKey) {
		t.Fatalf("bad key: %v", err)
	}
	got, err := svc.SaveVar(ctx, actor, access.DefaultWorkspaceRecID, settings.Var{Key: "FOO_BAR", Value: "1", IsEnabled: 1})
	if err != nil {
		t.Fatal(err)
	}
	if got.Key != "FOO_BAR" || got.Value != "1" {
		t.Fatalf("%+v", got)
	}
	list, err := svc.ListVars(ctx, actor, access.DefaultWorkspaceRecID)
	if err != nil || len(list) != 1 {
		t.Fatalf("list=%v err=%v", list, err)
	}
	env := svc.ProcEnv(ctx)
	if len(env) != 1 || env[0] != "FOO_BAR=1" {
		t.Fatalf("%v", env)
	}
	if err := svc.DeleteVar(ctx, actor, access.DefaultWorkspaceRecID, "FOO_BAR"); err != nil {
		t.Fatal(err)
	}
	list, err = svc.ListVars(ctx, actor, access.DefaultWorkspaceRecID)
	if err != nil || len(list) != 0 {
		t.Fatalf("after delete %v %v", list, err)
	}
}

func TestPropsWorkWithoutWorkspaceID(t *testing.T) {
	svc, ctx := setupSettings(t)
	actor := iam.Actor{UserID: access.DefaultAdminRecID, LoginName: "admin"}
	got, err := svc.GetProps(ctx, actor, "")
	if err != nil {
		t.Fatal(err)
	}
	if got.JobConcurrency != 6 {
		t.Fatalf("%+v", got)
	}
	saved, err := svc.SaveProps(ctx, actor, "", settings.Props{
		JobConcurrency:   3,
		LogRetentionDays: 7,
		WebTimeoutSec:    90,
	})
	if err != nil {
		t.Fatal(err)
	}
	if saved.JobConcurrency != 3 || saved.LogRetentionDays != 7 || saved.WebTimeoutSec != 90 {
		t.Fatalf("%+v", saved)
	}
}

func TestClampProps(t *testing.T) {
	p := settings.ClampProps(settings.Props{JobConcurrency: 0, LogRetentionDays: 400, WebTimeoutSec: 900})
	if p.JobConcurrency != 1 || p.LogRetentionDays != 365 || p.WebTimeoutSec != 600 {
		t.Fatalf("%+v", p)
	}
}
