package projectrelease

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIsWorkDirEmpty(t *testing.T) {
	dir := t.TempDir()
	empty, err := isWorkDirEmpty(dir)
	if err != nil || !empty {
		t.Fatalf("want empty dir, got empty=%v err=%v", empty, err)
	}
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	empty, err = isWorkDirEmpty(dir)
	if err != nil || empty {
		t.Fatalf("want non-empty, got empty=%v err=%v", empty, err)
	}
}

func TestSanitizeRepoURLForLog(t *testing.T) {
	got := sanitizeRepoURLForLog("https://oauth2:secret@git.example.com/a/b.git")
	if got != "https://git.example.com/a/b.git" {
		t.Fatalf("got %q", got)
	}
}
