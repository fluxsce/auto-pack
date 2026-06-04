package git

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"auto-pack/backend/pkg/shell"
)

// CloneOptions configures git clone.
type CloneOptions struct {
	Branch       string
	Depth        int
	SingleBranch bool
	Quiet        bool
}

// Clone clones url into dir. Parent directories of dir are created as needed.
func Clone(ctx context.Context, url, dir string, opts CloneOptions) (*Repo, error) {
	url = strings.TrimSpace(url)
	if url == "" {
		return nil, fmt.Errorf("git: empty remote url")
	}
	dir = filepath.Clean(dir)
	if dir == "" || dir == "." {
		return nil, fmt.Errorf("git: empty target directory")
	}
	if parent := filepath.Dir(dir); parent != "" && parent != "." {
		if err := os.MkdirAll(parent, 0o755); err != nil {
			return nil, fmt.Errorf("git: mkdir parent: %w", err)
		}
	}

	args := []string{"clone"}
	if opts.Quiet {
		args = append(args, "--quiet")
	}
	if opts.SingleBranch {
		args = append(args, "--single-branch")
	}
	if opts.Branch != "" {
		args = append(args, "--branch", opts.Branch)
	}
	if opts.Depth > 0 {
		args = append(args, "--depth", fmt.Sprintf("%d", opts.Depth))
	}
	args = append(args, url, dir)

	parent := filepath.Dir(dir)
	if parent == "" || parent == "." {
		parent, _ = os.Getwd()
	}
	if _, err := run(ctx, parent, args, shell.Options{}); err != nil {
		return nil, err
	}
	return &Repo{Dir: dir}, nil
}
