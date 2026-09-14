package runner

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"auto-pack/server/internal/job"
	"auto-pack/server/internal/project"
)

func TestJobContextEnv(t *testing.T) {
	j := job.Job{RecID: "job1", WorkspaceRecID: "ws1", ProjectRecID: "prj1", JobType: "build"}
	prj := project.Project{
		RecID:       "prj1",
		ProjectName: "demo-app",
		ProjectType: "maven",
		RepoURL:     "https://git.example/demo.git",
		RepoBranch:  "main",
	}
	wsPath := filepath.FromSlash("/data/ws/P05")
	projectPath := filepath.FromSlash("/data/ws/P05/demo-app")
	workdir := filepath.FromSlash("/data/ws/P05/demo-app/module")
	ctx := project.SourceContext{
		WorkspaceID: "ws1", WorkspaceCode: "P05", WorkspaceName: "P05 空间", WorkspacePath: wsPath,
		ProjectID: "prj1", ProjectName: "demo-app", ProjectType: "maven",
		JobID: "job1", JobType: "build", Tag: "V1.0.0",
		ProjectPath: projectPath, ProjectWorkdir: workdir,
		RepoBranch: "release/1.0", RepoURL: "https://git.example/demo.git",
	}
	got := jobContextEnv(j, prj, project.ParseRecipe(prj.RecipeJSON), ctx)
	m := map[string]string{}
	for _, pair := range got {
		i := strings.IndexByte(pair, '=')
		if i <= 0 {
			t.Fatalf("bad pair %q", pair)
		}
		m[pair[:i]] = pair[i+1:]
	}
	checks := map[string]string{
		"AP_JOB_ID":          "job1",
		"AP_JOB_TYPE":        "build",
		"AP_WORKSPACE_ID":    "ws1",
		"AP_WORKSPACE_CODE":  "P05",
		"AP_WORKSPACE_NAME":  "P05 空间",
		"AP_WORKSPACE_PATH":  wsPath,
		"AP_PROJECT_ID":      "prj1",
		"AP_PROJECT_NAME":    "demo-app",
		"AP_PROJECT_TYPE":    "maven",
		"AP_PROJECT_PATH":    projectPath,
		"AP_PROJECT_WORKDIR": workdir,
		"AP_REPO_BRANCH":     "release/1.0",
		"AP_REPO_URL":        "https://git.example/demo.git",
		"AP_TAG":             "V1.0.0",
	}
	for key, want := range checks {
		if m[key] != want {
			t.Fatalf("%s=%q want %q", key, m[key], want)
		}
	}
}

func TestStepPostShellInjectsContextEnv(t *testing.T) {
	var buf strings.Builder
	e := &Engine{}
	prj := project.Project{
		RecID:       "prj1",
		ProjectName: "ctx-demo",
		ProjectType: "custom",
		RecipeJSON:  `{"post_actions":[{"kind":"shell","command":"` + shellEchoProjectName() + `"}]}`,
	}
	j := job.Job{RecID: "job1", JobType: "build", ProjectRecID: "prj1"}
	if err := e.stepPost(t.Context(), j, prj, job.Payload{}, t.TempDir(), &buf, true); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "ctx-demo") {
		t.Fatalf("expected AP_PROJECT_NAME in output\n%s", buf.String())
	}
}

func shellEchoProjectName() string {
	if runtime.GOOS == "windows" {
		return "echo %AP_PROJECT_NAME%"
	}
	return "echo $AP_PROJECT_NAME"
}

func TestExpandProcEnvForLog(t *testing.T) {
	env := jobContextEnv(
		job.Job{RecID: "job1", WorkspaceRecID: "ws1", JobType: "build"},
		project.Project{RecID: "prj1", ProjectName: "demo-app", ProjectType: "maven"},
		project.Recipe{},
		project.SourceContext{
			WorkspaceID: "ws1", WorkspaceCode: "P05", WorkspaceName: "P05 空间", WorkspacePath: "/data/ws/P05",
			ProjectID: "prj1", ProjectName: "demo-app", ProjectType: "maven",
			JobID: "job1", JobType: "build", Tag: "V1.0.0",
			ProjectPath: "/data/ws/P05/demo-app", ProjectWorkdir: "/data/ws/P05/demo-app",
		},
	)
	cmd := `dest="/home/appinstall/${AP_WORKSPACE_CODE}/${AP_PROJECT_NAME}"`
	got := expandProcEnvForLog(cmd, env)
	want := `dest="/home/appinstall/P05/demo-app"`
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	if expandProcEnvForLog("echo $AP_PROJECT_NAME", env) != "echo demo-app" {
		t.Fatal("expected $AP_PROJECT_NAME expansion")
	}
	if expandProcEnvForLog("echo %AP_PROJECT_NAME%", env) != "echo demo-app" {
		t.Fatal("expected %AP_PROJECT_NAME% expansion")
	}
}

func TestJobContextEnvLocalSource(t *testing.T) {
	j := job.Job{RecID: "job1", WorkspaceRecID: "ws1", ProjectRecID: "prj1", JobType: "build"}
	abs := filepath.FromSlash("/home/appinstall/V10.0.0-P05")
	prj := project.Project{
		RecID:       "prj1",
		ProjectName: "local-app",
		ProjectType: "maven",
		RecipeJSON:  `{"scm_mode":"local","local_source_path":"/home/appinstall/V10.0.0-P05"}`,
	}
	recipe := project.ParseRecipe(prj.RecipeJSON)
	ctx := project.SourceContext{
		WorkspaceID: "ws1", WorkspaceCode: "P05", WorkspaceName: "P05 空间", WorkspacePath: "/data/ws/P05",
		ProjectID: "prj1", ProjectName: "local-app", ProjectType: "maven",
		JobID: "job1", JobType: "build",
		ProjectPath: abs, ProjectWorkdir: abs,
	}
	got := jobContextEnv(j, prj, recipe, ctx)
	m := map[string]string{}
	for _, pair := range got {
		i := strings.IndexByte(pair, '=')
		if i <= 0 {
			t.Fatalf("bad pair %q", pair)
		}
		m[pair[:i]] = pair[i+1:]
	}
	if m["AP_SCM_MODE"] != "local" {
		t.Fatalf("scm=%q", m["AP_SCM_MODE"])
	}
	wantLocal := filepath.ToSlash(abs)
	if m["AP_LOCAL_SOURCE_PATH"] != wantLocal {
		t.Fatalf("local=%q want %q", m["AP_LOCAL_SOURCE_PATH"], wantLocal)
	}
	if m["AP_PROJECT_PATH"] != abs {
		t.Fatalf("project path=%q want %q", m["AP_PROJECT_PATH"], abs)
	}
	cmd := `dest="${AP_LOCAL_SOURCE_PATH}"`
	if expandProcEnvForLog(cmd, got) != `dest="`+wantLocal+`"` {
		t.Fatalf("expand local source path failed")
	}
}

func TestJobContextEnvLocalSourceWithVars(t *testing.T) {
	j := job.Job{RecID: "job1", WorkspaceRecID: "ws1", ProjectRecID: "prj1", JobType: "build"}
	prj := project.Project{
		RecID:       "prj1",
		ProjectName: "demo-app",
		ProjectType: "maven",
		RecipeJSON:  `{"scm_mode":"local","local_source_path":"/home/appinstall/${AP_WORKSPACE_CODE}/${AP_PROJECT_NAME}"}`,
	}
	recipe := project.ParseRecipe(prj.RecipeJSON)
	ctx := project.SourceContext{
		WorkspaceID: "ws1", WorkspaceCode: "P05", ProjectName: "demo-app", ProjectType: "maven",
		JobID: "job1", JobType: "build",
		ProjectPath: "/home/appinstall/P05/demo-app", ProjectWorkdir: "/home/appinstall/P05/demo-app",
	}
	got := jobContextEnv(j, prj, recipe, ctx)
	m := map[string]string{}
	for _, pair := range got {
		i := strings.IndexByte(pair, '=')
		if i <= 0 {
			t.Fatalf("bad pair %q", pair)
		}
		m[pair[:i]] = pair[i+1:]
	}
	want := "/home/appinstall/P05/demo-app"
	if m["AP_LOCAL_SOURCE_PATH"] != want {
		t.Fatalf("local=%q want %q", m["AP_LOCAL_SOURCE_PATH"], want)
	}
}

func TestStepPostLogsExpandedShellCommand(t *testing.T) {
	var buf strings.Builder
	e := &Engine{}
	prj := project.Project{
		RecID:       "prj1",
		ProjectName: "ctx-demo",
		ProjectType: "custom",
		RecipeJSON:  `{"post_actions":[{"kind":"shell","command":"echo ${AP_PROJECT_NAME}"}]}`,
	}
	j := job.Job{RecID: "job1", JobType: "build", ProjectRecID: "prj1"}
	if err := e.stepPost(t.Context(), j, prj, job.Payload{}, t.TempDir(), &buf, true); err != nil {
		t.Fatal(err)
	}
	got := buf.String()
	if !strings.Contains(got, "执行命令 echo ctx-demo") {
		t.Fatalf("expected expanded command in log\n%s", got)
	}
	for _, line := range strings.Split(got, "\n") {
		if strings.Contains(line, "执行命令") && strings.Contains(line, "${AP_PROJECT_NAME}") {
			t.Fatalf("command log should not contain placeholder: %s", line)
		}
	}
}
