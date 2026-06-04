package jdk

import (
	"os"
	"path/filepath"
	"testing"

	"auto-pack/backend/pkg/envtypes"
)

func TestNormalizeJavaPath_directoryBin(t *testing.T) {
	javaName := javaExecutableName()
	root := t.TempDir()
	binDir := filepath.Join(root, "bin")
	if err := os.Mkdir(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	got := normalizeJavaPath(binDir, javaName)
	want := filepath.Join(binDir, javaName)
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestNormalizeJavaPath_javaHome(t *testing.T) {
	javaName := javaExecutableName()
	home := t.TempDir()
	got := normalizeJavaPath(home, javaName)
	want := filepath.Join(home, "bin", javaName)
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestResolveJavaBin_prefersJavaPathOverHome(t *testing.T) {
	root := t.TempDir()
	binDir := filepath.Join(root, "bin")
	if err := os.Mkdir(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := envtypes.JDKConfig{JavaHome: root, JavaPath: binDir}
	got := resolveJavaBin(cfg)
	want := filepath.Join(binDir, javaExecutableName())
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}
