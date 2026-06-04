package projectfs

import (
	"path/filepath"
	"testing"

	"auto-pack/backend/internal/repository"
	"auto-pack/backend/pkg/projectcfg"
)

func TestResolveProjectAbsDirUnderWorkspace(t *testing.T) {
	data := filepath.Join("app", "data")
	project := &repository.PackProject{WorkDir: "demo-app"}
	ws := &repository.PackWorkspace{
		WorkspaceCode: "default",
		RootDir:       "",
	}
	got := ResolveProjectAbsDir(project, ws, data)
	want := filepath.Join(data, projectcfg.WorkspaceDataSubdir, "default", "demo-app")
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestResolveProjectAbsDirCustomWorkspaceRoot(t *testing.T) {
	data := filepath.Join("app", "data")
	custom := filepath.Join("D:", "pack", "team-a")
	project := &repository.PackProject{WorkDir: "demo-app"}
	ws := &repository.PackWorkspace{
		WorkspaceCode: "team-a",
		RootDir:       custom,
	}
	got := ResolveProjectAbsDir(project, ws, data)
	want := filepath.Join(custom, "demo-app")
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}
