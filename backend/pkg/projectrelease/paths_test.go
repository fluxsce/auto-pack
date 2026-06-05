package projectrelease

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestEnsureResourceFiles_createsMissing(t *testing.T) {
	dir := t.TempDir()
	versionPath, updateNotesPath, ok := ResolveResourcePaths(dir)
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

func TestBuildVersionContent_emptyNotes(t *testing.T) {
	dir := t.TempDir()
	versionPath := filepath.Join(dir, filepath.FromSlash(relVersionFile))
	updateNotesPath := filepath.Join(dir, filepath.FromSlash(relUpdateNotesFile))
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
