package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWorkspaceRootDefault(t *testing.T) {
	dir := t.TempDir()
	cfgDir := filepath.Join(dir, "configs")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := []byte("server:\n  addr: \":0\"\n")
	if err := os.WriteFile(filepath.Join(cfgDir, "app.yaml"), body, 0o644); err != nil {
		t.Fatal(err)
	}
	old := configDirFlag
	configDirFlag = cfgDir
	t.Cleanup(func() { configDirFlag = old })
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(dir, "data", "workspaces")
	if cfg.WorkspaceRoot != want {
		t.Fatalf("root=%s want=%s", cfg.WorkspaceRoot, want)
	}
}

func TestWorkspaceRootAbsolute(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, "packs")
	cfgDir := filepath.Join(dir, "configs")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := []byte("workspace:\n  root: \"" + filepath.ToSlash(root) + "\"\n")
	if err := os.WriteFile(filepath.Join(cfgDir, "app.yaml"), body, 0o644); err != nil {
		t.Fatal(err)
	}
	old := configDirFlag
	configDirFlag = cfgDir
	t.Cleanup(func() { configDirFlag = old })
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if filepath.ToSlash(filepath.Clean(cfg.WorkspaceRoot)) != filepath.ToSlash(filepath.Clean(root)) {
		t.Fatalf("root=%s want=%s", cfg.WorkspaceRoot, root)
	}
}

func TestLogFileResolvesAgainstAppRoot(t *testing.T) {
	dir := t.TempDir()
	cfgDir := filepath.Join(dir, "configs")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := []byte("log:\n  file: ./logs/autopack.log\n")
	if err := os.WriteFile(filepath.Join(cfgDir, "app.yaml"), body, 0o644); err != nil {
		t.Fatal(err)
	}
	old := configDirFlag
	configDirFlag = cfgDir
	t.Cleanup(func() { configDirFlag = old })
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(dir, "logs", "autopack.log")
	if cfg.LogFile != want {
		t.Fatalf("log=%s want=%s", cfg.LogFile, want)
	}
}
