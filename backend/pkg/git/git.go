package git

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"auto-pack/backend/pkg/shell"
)

// ErrNotRepository indicates the path is not inside a git work tree.
var ErrNotRepository = errors.New("git: not a git repository")

// ErrNotFound is returned when git cannot resolve a ref or path.
var ErrNotFound = errors.New("git: not found")

// Output is the combined stdout/stderr from a git invocation.
type Output struct {
	Stdout string
	Stderr string
}

var (
	exeOnce sync.Once
	exePath string
	exeErr  error
)

// Executable returns the resolved git binary path.
func Executable() (string, error) {
	exeOnce.Do(func() {
		exePath, exeErr = resolveExecutable()
	})
	return exePath, exeErr
}

func resolveExecutable() (string, error) {
	if p := strings.TrimSpace(os.Getenv("GIT")); p != "" {
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}
	if p, err := exec.LookPath("git"); err == nil {
		return p, nil
	}
	return "", fmt.Errorf("git: executable not found in PATH")
}

// IsRepository reports whether dir is inside a git work tree.
func IsRepository(ctx context.Context, dir string) bool {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return false
	}
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return false
	}
	_, err = run(ctx, dir, []string{"rev-parse", "--is-inside-work-tree"}, shell.Options{})
	return err == nil
}

func run(ctx context.Context, dir string, args []string, opts shell.Options) (Output, error) {
	exe, err := Executable()
	if err != nil {
		return Output{}, err
	}
	if dir != "" {
		dir = filepath.Clean(dir)
		opts.Dir = dir
	}
	res, err := shell.RunArgs(ctx, exe, args, opts)
	out := Output{Stdout: res.Stdout, Stderr: res.Stderr}
	if err != nil {
		if isNotFound(stderrOrOut(res)) {
			return out, ErrNotFound
		}
		return out, fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return out, nil
}

func stderrOrOut(res shell.Result) string {
	if s := strings.TrimSpace(res.Stderr); s != "" {
		return s
	}
	return res.Stdout
}

func isNotFound(msg string) bool {
	lower := strings.ToLower(msg)
	return strings.Contains(lower, "not a valid object") ||
		strings.Contains(lower, "unknown revision") ||
		strings.Contains(lower, "bad revision") ||
		strings.Contains(lower, "pathspec") && strings.Contains(lower, "did not match")
}

func trimLine(s string) string {
	return strings.TrimSpace(s)
}

func lines(s string) []string {
	raw := strings.Split(strings.TrimRight(s, "\r\n"), "\n")
	out := make([]string, 0, len(raw))
	for _, line := range raw {
		line = strings.TrimRight(line, "\r")
		if line != "" {
			out = append(out, line)
		}
	}
	return out
}
