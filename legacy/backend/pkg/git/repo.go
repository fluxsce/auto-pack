package git

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"auto-pack/backend/pkg/shell"
)

// Repo operates on a single working tree directory.
type Repo struct {
	Dir string
}

// Open opens an existing repository at dir (may be a subdirectory of the work tree).
func Open(dir string) (*Repo, error) {
	dir = filepath.Clean(dir)
	if !filepath.IsAbs(dir) {
		abs, err := filepath.Abs(dir)
		if err != nil {
			return nil, fmt.Errorf("git: resolve path: %w", err)
		}
		dir = abs
	}
	if _, err := os.Stat(dir); err != nil {
		return nil, fmt.Errorf("git: %w", err)
	}
	ctx := context.Background()
	if !IsRepository(ctx, dir) {
		return nil, ErrNotRepository
	}
	return &Repo{Dir: dir}, nil
}

// Init creates a new repository at dir (directory must exist or will be created).
func Init(ctx context.Context, dir string, bare bool) (*Repo, error) {
	dir = filepath.Clean(dir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("git: mkdir: %w", err)
	}
	args := []string{"init"}
	if bare {
		args = append(args, "--bare")
	}
	if _, err := run(ctx, dir, args, shell.Options{}); err != nil {
		return nil, err
	}
	return &Repo{Dir: dir}, nil
}

// Run executes arbitrary git subcommands in this repository.
func (r *Repo) Run(ctx context.Context, args ...string) (Output, error) {
	return run(ctx, r.Dir, args, shell.Options{})
}

// Root returns the absolute path to the work tree root.
func (r *Repo) Root(ctx context.Context) (string, error) {
	out, err := run(ctx, r.Dir, []string{"rev-parse", "--show-toplevel"}, shell.Options{})
	if err != nil {
		return "", err
	}
	return filepath.Clean(trimLine(out.Stdout)), nil
}

// GitDir returns the absolute path to the .git directory.
func (r *Repo) GitDir(ctx context.Context) (string, error) {
	out, err := run(ctx, r.Dir, []string{"rev-parse", "--git-dir"}, shell.Options{})
	if err != nil {
		return "", err
	}
	p := trimLine(out.Stdout)
	if !filepath.IsAbs(p) {
		p = filepath.Join(r.Dir, p)
	}
	return filepath.Clean(p), nil
}
