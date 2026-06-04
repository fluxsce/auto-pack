package shell

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestRunEcho(t *testing.T) {
	res, err := Run(context.Background(), "echo hello-shell", Options{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(res.Stdout, "hello-shell") {
		t.Fatalf("stdout = %q, want hello-shell", res.Stdout)
	}
	if res.ExitCode != 0 {
		t.Fatalf("exit = %d, want 0", res.ExitCode)
	}
}

func TestRunEmptyScript(t *testing.T) {
	_, err := Run(context.Background(), "  ", Options{})
	if err == nil {
		t.Fatal("expected error for empty script")
	}
}

func TestRunNonZeroExit(t *testing.T) {
	_, err := Run(context.Background(), "exit 42", Options{})
	if err == nil {
		t.Fatal("expected ExitError")
	}
	var exitErr *ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("got %T, want *ExitError", err)
	}
	if exitErr.Result.ExitCode != 42 {
		t.Fatalf("exit = %d, want 42", exitErr.Result.ExitCode)
	}
}

func TestRunAllowNonZeroExit(t *testing.T) {
	res, err := Run(context.Background(), "exit 7", Options{AllowNonZeroExit: true})
	if err != nil {
		t.Fatal(err)
	}
	if res.ExitCode != 7 {
		t.Fatalf("exit = %d, want 7", res.ExitCode)
	}
}

func TestRunArgs(t *testing.T) {
	ctx := context.Background()
	name := "echo"
	args := []string{"direct-args"}
	if runtime.GOOS == "windows" {
		name = ShellExecutable()
		args = []string{"/C", "echo direct-args"}
	}

	res, err := RunArgs(ctx, name, args, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(res.Stdout, "direct-args") {
		t.Fatalf("stdout = %q", res.Stdout)
	}
}

func TestRunWithDir(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()

	var script string
	if runtime.GOOS == "windows" {
		script = "cd"
	} else {
		script = "pwd"
	}

	res, err := Run(ctx, script, Options{Dir: dir})
	if err != nil {
		t.Fatal(err)
	}
	got := strings.TrimSpace(res.Stdout)
	want := filepath.Clean(dir)
	if runtime.GOOS == "windows" {
		if !strings.EqualFold(got, want) {
			t.Fatalf("stdout = %q, want %q", got, want)
		}
		return
	}
	if got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
}

func TestRunContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := Run(ctx, "echo x", Options{})
	if err == nil {
		t.Fatal("expected context error")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("got %v, want context.Canceled", err)
	}
}

func TestRunContextTimeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	var script string
	if runtime.GOOS == "windows" {
		script = "timeout /t 5 /nobreak >nul"
	} else {
		script = "sleep 5"
	}

	_, err := Run(ctx, script, Options{})
	if err == nil {
		t.Fatal("expected timeout/cancel error")
	}
}

func TestShellLabel(t *testing.T) {
	label := ShellLabel()
	if label == "" {
		t.Fatal("ShellLabel returned empty")
	}
	if runtime.GOOS == "windows" && !strings.Contains(label, "cmd") {
		t.Fatalf("label = %q", label)
	}
	if runtime.GOOS != "windows" && !strings.Contains(label, "sh") {
		t.Fatalf("label = %q", label)
	}
}

func TestRunWithEnv(t *testing.T) {
	const key = "AUTO_PACK_SHELL_TEST"
	const value = "xyzzy"
	ctx := context.Background()

	var script string
	if runtime.GOOS == "windows" {
		script = "echo %" + key + "%"
	} else {
		script = "echo $" + key
	}

	res, err := Run(ctx, script, Options{Env: []string{key + "=" + value}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(res.Stdout, value) {
		t.Fatalf("stdout = %q, want %q", res.Stdout, value)
	}
	_ = os.Unsetenv(key)
}
