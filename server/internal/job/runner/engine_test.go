package runner

import (
	"context"
	"strings"
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
	if skip, reason := skipSync(project.Project{
		RepoURL:    "https://git.example.com/a.git",
		RecipeJSON: `{"scm_mode":"git"}`,
	}, job.Payload{SkipSync: true}); !skip || !strings.Contains(reason, "already synced") {
		t.Fatalf("skip_sync %v %q", skip, reason)
	}
	if originLabel(job.OriginAutoBuild) != "工作空间自动构建" {
		t.Fatal(originLabel(job.OriginAutoBuild))
	}
}

func TestSkipDependentEnqueue(t *testing.T) {
	if !skipDependentEnqueue(job.Job{PayloadJSON: `{"origin":"auto_build"}`}) {
		t.Fatal("auto_build")
	}
	if !skipDependentEnqueue(job.Job{PayloadJSON: `{"origin":"auto_release"}`}) {
		t.Fatal("auto_release")
	}
	if skipDependentEnqueue(job.Job{PayloadJSON: `{}`}) {
		t.Fatal("manual")
	}
	e := NewEngine(nil, nil, nil, t.TempDir(), "", 0)
	called := 0
	e.submit = func(context.Context, job.SubmitRequest) (job.Job, error) {
		called++
		return job.Job{}, nil
	}
	var buf strings.Builder
	e.afterSuccess(context.Background(), job.Job{
		JobType:     job.TypeBuild,
		PayloadJSON: `{"origin":"auto_build"}`,
	}, &buf)
	if called != 0 {
		t.Fatalf("auto_build should not enqueue dependents, called=%d", called)
	}
}

func TestSkipBuildAsSuccess(t *testing.T) {
	if !skipBuildAsSuccess("custom", "") || !skipBuildAsSuccess("customer", "  ") {
		t.Fatal("empty custom should skip")
	}
	if skipBuildAsSuccess("custom", "echo ok") {
		t.Fatal("filled custom command still runs")
	}
	if skipBuildAsSuccess("maven", "") || skipBuildAsSuccess("go", "") {
		t.Fatal("other types still require a command")
	}
	e := NewEngine(nil, nil, nil, t.TempDir(), "", 0)
	var buf strings.Builder
	if err := e.stepBuild(context.Background(), job.Job{}, project.Project{
		ProjectType: "custom",
		RecipeJSON:  `{"command":""}`,
	}, job.Payload{}, t.TempDir(), &buf); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "跳过") {
		t.Fatalf("log=%s", buf.String())
	}
	if err := e.stepBuild(context.Background(), job.Job{}, project.Project{
		ProjectType: "go",
		RecipeJSON:  `{"command":""}`,
	}, job.Payload{}, t.TempDir(), &buf); err == nil {
		t.Fatal("empty go command should fail")
	}
}

func TestEnvCfgFallsBackToKind(t *testing.T) {
	e := NewEngine(nil, nil, nil, t.TempDir(), "", 1)
	if cfg := e.envCfg(context.Background(), "missing", "ws", "jdk"); cfg != nil {
		t.Fatal(cfg)
	}
}

func TestEnvCfgEmptyJDKDoesNotFallback(t *testing.T) {
	e := NewEngine(nil, nil, nil, t.TempDir(), "", 1)
	if cfg := e.envCfg(context.Background(), "", "ws", "jdk"); cfg != nil {
		t.Fatal(cfg)
	}
}

func TestEnvValuePrefersLast(t *testing.T) {
	if got := envValue([]string{"JAVA_HOME=/old", "JAVA_HOME=/jdk8"}, "JAVA_HOME"); got != "/jdk8" {
		t.Fatal(got)
	}
	if envValue(nil, "JAVA_HOME") != "" {
		t.Fatal("empty")
	}
}
