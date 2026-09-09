package runner

import (
	"context"
	"testing"

	"auto-pack/server/internal/job"
	"auto-pack/server/internal/project"
)

func TestKindCfgNilService(t *testing.T) {
	e := NewEngine(nil, nil, nil, t.TempDir(), "", 1)
	if cfg := e.kindCfg(context.Background(), "ws", "git"); cfg != nil {
		t.Fatal(cfg)
	}
	opt := e.gitOpt(context.Background(), "ws")
	if opt.Depth != 1 || opt.Auth.Token != "" {
		t.Fatalf("%+v", opt)
	}
	_, _, err := e.uploadEnv(context.Background(), "ws", "")
	if err == nil {
		t.Fatal("expected env service error")
	}
}

func TestNotifySkipsWithoutRecipients(t *testing.T) {
	e := NewEngine(nil, nil, nil, t.TempDir(), "", 0)
	e.notify(context.Background(), job.Job{WorkspaceRecID: "ws", RecID: "j1", JobType: job.TypeBuild}, true, "", nil)
}

func TestSkipSync(t *testing.T) {
	if skip, _ := skipSync(project.Project{RecipeJSON: `{"scm_mode":"local"}`}, job.Payload{}); !skip {
		t.Fatal("local")
	}
	if skip, _ := skipSync(project.Project{
		RepoURL:    "https://git.example.com/a.git",
		RecipeJSON: `{"scm_mode":"git","auto_sync":false}`,
	}, job.Payload{}); !skip {
		t.Fatal("auto_sync")
	}
	if skip, _ := skipSync(project.Project{RecipeJSON: `{"scm_mode":"git"}`}, job.Payload{}); !skip {
		t.Fatal("no url")
	}
	if skip, _ := skipSync(project.Project{
		RepoURL:    "https://git.example.com/a.git",
		RecipeJSON: `{"scm_mode":"git"}`,
	}, job.Payload{}); skip {
		t.Fatal("should clone")
	}
}

func TestEnvCfgFallsBackToKind(t *testing.T) {
	e := NewEngine(nil, nil, nil, t.TempDir(), "", 1)
	if cfg := e.envCfg(context.Background(), "missing", "ws", "jdk"); cfg != nil {
		t.Fatal(cfg)
	}
}
