package packrun

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureProjectParentDir(t *testing.T) {
	root := t.TempDir()
	workDir := filepath.Join(root, "workspaces", "default", "demo-app")

	if err := ensureProjectParentDir(workDir); err != nil {
		t.Fatal(err)
	}
	parent := filepath.Dir(workDir)
	if st, err := os.Stat(parent); err != nil || !st.IsDir() {
		t.Fatalf("parent dir missing: %v", err)
	}
	if _, err := os.Stat(workDir); !os.IsNotExist(err) {
		t.Fatal("work dir should not be created by ensureProjectParentDir")
	}
}
