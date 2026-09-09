package projectfs

import (
	"os"
	"path/filepath"
	"testing"
	"time"

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

func TestResolveProjectSourceAbsDirFallsBackToWorkDir(t *testing.T) {
	data := filepath.Join("app", "data")
	project := &repository.PackProject{
		WorkDir:    "shared-datahub",
		ConfigJSON: `{"scm_mode":"local"}`,
	}
	ws := &repository.PackWorkspace{
		WorkspaceCode: "V10.0.0-P02",
		RootDir:       "",
	}
	got := ResolveProjectSourceAbsDir(project, ws, data)
	want := ResolveProjectAbsDir(project, ws, data)
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestResolveProjectSourceAbsDirRelativeOverride(t *testing.T) {
	data := filepath.Join("app", "data")
	custom := filepath.Join("D:", "pack", "team-a")
	project := &repository.PackProject{
		WorkDir:    "shared-datahub",
		ConfigJSON: `{"scm_mode":"local","local_source_path":"sources/shared-datahub"}`,
	}
	ws := &repository.PackWorkspace{
		WorkspaceCode: "team-a",
		RootDir:       custom,
	}
	got := ResolveProjectSourceAbsDir(project, ws, data)
	want := filepath.Join(custom, "sources", "shared-datahub")
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestListDirIncludesModTime(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "hello.txt"), []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}
	listing, err := ListDir(dir, ".")
	if err != nil {
		t.Fatal(err)
	}
	if len(listing.Entries) != 1 {
		t.Fatalf("entries %d", len(listing.Entries))
	}
	e := listing.Entries[0]
	if e.Name != "hello.txt" || e.Size != 2 || e.IsDir {
		t.Fatalf("entry %+v", e)
	}
	if e.ModTime == "" {
		t.Fatal("missing mod_time")
	}
	if _, err := time.Parse(time.RFC3339, e.ModTime); err != nil {
		t.Fatalf("mod_time %q: %v", e.ModTime, err)
	}
}
