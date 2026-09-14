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
	p := job.Payload{Branch: "release/1.0", Tag: "V1.0.0"}
	wsPath := filepath.FromSlash("/data/ws/P05")
	projectPath := filepath.FromSlash("/data/ws/P05/demo-app")
	workdir := filepath.FromSlash("/data/ws/P05/demo-app/module")
	got := jobContextEnv(j, prj, p, "P05", "P05 空间", wsPath, projectPath, workdir)
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
