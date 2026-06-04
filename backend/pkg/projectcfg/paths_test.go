package projectcfg

import (
	"path/filepath"
	"testing"
)

func TestResolveWorkspaceRoot(t *testing.T) {
	data := filepath.Join("app", "data")
	custom := filepath.Join("workspaces", "prod")
	if got := ResolveWorkspaceRoot(custom, data, "ignored"); got != custom {
		t.Fatalf("custom root: got %q want %q", got, custom)
	}
	wantDefault := filepath.Join(data, WorkspaceDataSubdir, "default")
	if got := ResolveWorkspaceRoot("", data, "default"); got != wantDefault {
		t.Fatalf("empty root: got %q want %q", got, wantDefault)
	}
	if got := ResolveWorkspaceRoot("", data, ""); got != wantDefault {
		t.Fatalf("empty root and code: got %q want %q", got, wantDefault)
	}
	wantProd := filepath.Join(data, WorkspaceDataSubdir, "prod")
	if got := ResolveWorkspaceRoot("", data, "prod"); got != wantProd {
		t.Fatalf("prod workspace: got %q want %q", got, wantProd)
	}
}

func TestJoinWorkDirUsesWorkspaceSubdirWhenRootEmpty(t *testing.T) {
	data := filepath.Join("app", "data")
	got := JoinWorkDir("", data, "default", "my-app")
	want := filepath.Join(data, WorkspaceDataSubdir, "default", "my-app")
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestJoinWorkDirUsesCustomWorkspaceRoot(t *testing.T) {
	data := filepath.Join("app", "data")
	custom := filepath.Join("D:", "pack", "prod")
	got := JoinWorkDir(custom, data, "default", "my-app")
	want := filepath.Join(custom, "my-app")
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}
