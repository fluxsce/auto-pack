package projectrelease

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveResourcePaths_maven(t *testing.T) {
	dir := t.TempDir()
	versionPath, updateNotesPath, ok := ResolveResourcePaths(dir, "maven")
	if !ok {
		t.Fatal("resolve failed")
	}
	wantVersion := filepath.Join(dir, filepath.FromSlash(relMavenVersionFile))
	wantNotes := filepath.Join(dir, filepath.FromSlash(relMavenUpdateNotesFile))
	if versionPath != wantVersion || updateNotesPath != wantNotes {
		t.Fatalf("got version=%q notes=%q", versionPath, updateNotesPath)
	}
}

func TestResolveResourcePaths_node(t *testing.T) {
	dir := t.TempDir()
	versionPath, updateNotesPath, ok := ResolveResourcePaths(dir, "vue")
	if !ok {
		t.Fatal("resolve failed")
	}
	wantVersion := filepath.Join(dir, filepath.FromSlash(relNodeVersionFile))
	wantNotes := filepath.Join(dir, filepath.FromSlash(relNodeUpdateNotesFile))
	if versionPath != wantVersion || updateNotesPath != wantNotes {
		t.Fatalf("got version=%q notes=%q", versionPath, updateNotesPath)
	}
}

func TestResolveResourcePaths_unsupported(t *testing.T) {
	dir := t.TempDir()
	_, _, ok := ResolveResourcePaths(dir, "go")
	if ok {
		t.Fatal("expected unsupported type to fail")
	}
}

func TestEnsureResourceFiles_createsMissing(t *testing.T) {
	dir := t.TempDir()
	versionPath, updateNotesPath, ok := ResolveResourcePaths(dir, "maven")
	if !ok {
		t.Fatal("resolve failed")
	}
	created, err := EnsureResourceFiles(versionPath, updateNotesPath, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(created) != 2 {
		t.Fatalf("created %v", created)
	}
	if !fileExists(versionPath) || !fileExists(updateNotesPath) {
		t.Fatal("files should exist")
	}
	created2, err := EnsureResourceFiles(versionPath, updateNotesPath, nil)
	if err != nil || len(created2) != 0 {
		t.Fatalf("second ensure: created=%v err=%v", created2, err)
	}
}

func TestBuildVersionContent_emptyNotes_maven(t *testing.T) {
	dir := t.TempDir()
	versionPath, updateNotesPath, ok := ResolveResourcePaths(dir, "maven")
	if !ok {
		t.Fatal("resolve failed")
	}
	if _, err := EnsureResourceFiles(versionPath, updateNotesPath, nil); err != nil {
		t.Fatal(err)
	}
	out, err := BuildVersionContent(versionPath, updateNotesPath, SyncOptions{
		Tag:         "V9.0.0-P01-01",
		ReleaseDate: "2026-06-04",
		AllowRepeat: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, part := range []string{"Version:", "V9.0.0-P01-01", "2026-06-04"} {
		if !strings.Contains(out, part) {
			t.Fatalf("missing %q in:\n%s", part, out)
		}
	}
}

func TestBuildVersionContent_nodeFromUpdateNotes(t *testing.T) {
	dir := t.TempDir()
	versionPath, updateNotesPath, ok := ResolveResourcePaths(dir, "npm")
	if !ok {
		t.Fatal("resolve failed")
	}
	if _, err := EnsureResourceFiles(versionPath, updateNotesPath, nil); err != nil {
		t.Fatal(err)
	}
	notes := "20260604 shangjian 【功能完善】前端列表优化，任务号:20260127095017000183\n"
	if err := os.WriteFile(updateNotesPath, []byte(notes), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := BuildVersionContent(versionPath, updateNotesPath, SyncOptions{
		Tag:         "V9.0.0-P01-01",
		ReleaseDate: "2026-06-04",
		AllowRepeat: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "前端列表优化") {
		t.Fatalf("missing note summary in:\n%s", out)
	}
}
