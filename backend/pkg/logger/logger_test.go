package logger

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultOptions(t *testing.T) {
	o := DefaultOptions()
	if o.MaxSizeMB != DefaultMaxSizeMB {
		t.Fatalf("MaxSizeMB = %d, want %d", o.MaxSizeMB, DefaultMaxSizeMB)
	}
	if o.MaxBackups != DefaultMaxBackups {
		t.Fatalf("MaxBackups = %d, want %d", o.MaxBackups, DefaultMaxBackups)
	}
}

func TestInitFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "app.log")

	if err := Init(Options{File: path, Level: "debug", AlsoStdout: false}); err != nil {
		t.Fatal(err)
	}
	Info("hello", "k", "v")

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("log file missing: %v", err)
	}

	t.Cleanup(func() {
		_ = Close()
		_ = Init(Options{Level: "info"})
	})
}

func TestInitInvalidLevel(t *testing.T) {
	if err := Init(Options{Level: "verbose"}); err == nil {
		t.Fatal("expected error for invalid level")
	}
}

func TestLNotNil(t *testing.T) {
	if L() == nil {
		t.Fatal("L() returned nil")
	}
	_ = slog.LevelInfo
}
