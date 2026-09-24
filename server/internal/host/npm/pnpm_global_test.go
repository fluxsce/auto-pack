package npm

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestStripGlobalPnpmInstall(t *testing.T) {
	in := "npm install -g pnpm\npnpm  install\npnpm run build"
	got, ok := stripGlobalPnpmInstall(in)
	if !ok {
		t.Fatal("expected strip")
	}
	if got != "pnpm  install\npnpm run build" {
		t.Fatalf("got %q", got)
	}
	if _, ok := stripGlobalPnpmInstall("pnpm install\npnpm run build"); ok {
		t.Fatal("plain pnpm script must stay")
	}
	got, ok = stripGlobalPnpmInstall("npm i --global pnpm@9 && pnpm run build")
	if !ok || got != "pnpm run build" {
		t.Fatalf("inline got %q ok=%v", got, ok)
	}
	got, ok = stripGlobalPnpmInstall("npm install -g pnpm; pnpm install")
	if !ok || got != "pnpm install" {
		t.Fatalf("semicolon got %q ok=%v", got, ok)
	}
}

func TestGlobalPnpmSpec(t *testing.T) {
	spec, ok := globalPnpmSpec("npm install -g pnpm\npnpm install")
	if !ok || spec != "" {
		t.Fatalf("unversioned spec=%q ok=%v", spec, ok)
	}
	spec, ok = globalPnpmSpec("npm install -g pnpm@10.1.0 && pnpm run build")
	if !ok || spec != "10.1.0" {
		t.Fatalf("pinned spec=%q ok=%v", spec, ok)
	}
	spec, ok = globalPnpmSpec("npm i --global pnpm@v9")
	if !ok || spec != "9" {
		t.Fatalf("major spec=%q ok=%v", spec, ok)
	}
}

func TestVersionSatisfied(t *testing.T) {
	if !versionSatisfied("9.15.0", "9") || !versionSatisfied("9.15.0", "9.15") || !versionSatisfied("9.15.0", "9.15.0") {
		t.Fatal("prefix should match")
	}
	if versionSatisfied("9.15.0", "10") || versionSatisfied("9.15.0", "9.15.1") || versionSatisfied("9.15", "9.15.0") {
		t.Fatal("different version should not match")
	}
	if !versionSatisfied("10.0.0", "") || !versionSatisfied("10.0.0", "latest") {
		t.Fatal("empty or latest accepts any install")
	}
}

func TestIsGlobalPnpmInstall(t *testing.T) {
	if !isGlobalPnpmInstall("npm install -g pnpm") {
		t.Fatal("install -g")
	}
	if isGlobalPnpmInstall("pnpm install") || isGlobalPnpmInstall("npm install pnpm") {
		t.Fatal("local install is not global")
	}
}

func TestPnpmOnPathUsesInjectedPath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "pnpm")
	body := []byte("#!/bin/sh\n")
	mode := os.FileMode(0o755)
	if runtime.GOOS == "windows" {
		path += ".cmd"
		body = []byte("@echo off\r\n")
	}
	if err := os.WriteFile(path, body, mode); err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS == "windows" {
		t.Skip("pnpmOnPath looks up the name pnpm without PATHEXT")
	}
	if !pnpmOnPath([]string{"PATH=" + dir}) {
		t.Fatal("expected pnpm on injected PATH")
	}
	if pnpmOnPath([]string{"PATH=" + t.TempDir()}) {
		t.Fatal("empty dir must not count")
	}
}
