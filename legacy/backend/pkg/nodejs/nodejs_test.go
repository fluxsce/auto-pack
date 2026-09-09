package nodejs

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNormalizeBinPath_installDir(t *testing.T) {
	root := t.TempDir()
	nodeExe := filepath.Join(root, "node.exe")
	if err := os.WriteFile(nodeExe, []byte{}, 0o755); err != nil {
		t.Fatal(err)
	}
	got := normalizeBinPath(root, "node")
	if got != nodeExe {
		t.Fatalf("got %q want %q", got, nodeExe)
	}
}

func TestNormalizeBinPath_nodeExePath(t *testing.T) {
	root := t.TempDir()
	nodeExe := filepath.Join(root, "node.exe")
	if err := os.WriteFile(nodeExe, []byte{}, 0o755); err != nil {
		t.Fatal(err)
	}
	got := normalizeBinPath(nodeExe, "node")
	if got != nodeExe {
		t.Fatalf("got %q want %q", got, nodeExe)
	}
}

func TestResolveBinPath_installDir(t *testing.T) {
	root := t.TempDir()
	nodeExe := filepath.Join(root, "node.exe")
	if err := os.WriteFile(nodeExe, []byte{}, 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := ResolveBinPath(root, "node")
	if err != nil {
		t.Fatal(err)
	}
	if got != nodeExe {
		t.Fatalf("got %q want %q", got, nodeExe)
	}
}
