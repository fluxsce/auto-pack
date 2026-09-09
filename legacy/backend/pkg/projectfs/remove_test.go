package projectfs

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRemovePath_entireBase(t *testing.T) {
	base := filepath.Join(t.TempDir(), "proj")
	if err := os.MkdirAll(filepath.Join(base, "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(base, "src", "a.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := RemovePath(base, "."); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(base); !os.IsNotExist(err) {
		t.Fatalf("expected base removed, stat err=%v", err)
	}
}

func TestRemovePath_childEntry(t *testing.T) {
	base := filepath.Join(t.TempDir(), "proj")
	child := filepath.Join(base, "tmp")
	if err := os.MkdirAll(child, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := RemovePath(base, "tmp"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(base); err != nil {
		t.Fatalf("base should remain: %v", err)
	}
	if _, err := os.Stat(child); !os.IsNotExist(err) {
		t.Fatalf("child should be removed")
	}
}

func TestRemovePath_notFound(t *testing.T) {
	base := t.TempDir()
	err := RemovePath(base, "missing")
	if err != ErrNotFound {
		t.Fatalf("got %v want ErrNotFound", err)
	}
}

func TestRemovePath_traversalRejected(t *testing.T) {
	base := t.TempDir()
	err := RemovePath(base, "../escape")
	if err != ErrPathTraversal {
		t.Fatalf("got %v want ErrPathTraversal", err)
	}
}
