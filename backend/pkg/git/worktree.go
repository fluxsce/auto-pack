package git

import (
	"context"
	"fmt"
	"strings"

	"auto-pack/backend/pkg/shell"
)

// WorktreeStatus summarizes git status --porcelain=v1 -b.
type WorktreeStatus struct {
	Branch       string
	Upstream     string
	Ahead        int
	Behind       int
	Clean        bool
	Modified     []string
	Added        []string
	Deleted      []string
	Renamed      []string
	Copied       []string
	Untracked    []string
	Ignored      []string
	Unmerged     []string
	Detached     bool
	RawPorcelain string
}

// PullOptions configures git pull.
type PullOptions struct {
	Remote string
	Branch string
	Rebase bool
	FFOnly bool
}

// FetchOptions configures git fetch.
type FetchOptions struct {
	Remote string
	Prune  bool
	Tags   bool
	Depth  int
}

// CheckoutOptions configures git checkout.
type CheckoutOptions struct {
	CreateBranch bool
	Force        bool
}

// CommitOptions configures git commit.
type CommitOptions struct {
	AllowEmpty bool
	All        bool
}

// Status returns parsed porcelain status.
func (r *Repo) Status(ctx context.Context) (WorktreeStatus, error) {
	out, err := run(ctx, r.Dir, []string{"status", "--porcelain=v1", "-b"}, shell.Options{})
	if err != nil {
		return WorktreeStatus{}, err
	}
	return parsePorcelainStatus(out.Stdout), nil
}

// Pull runs git pull in the repository.
func (r *Repo) Pull(ctx context.Context, opts PullOptions) error {
	args := []string{"pull"}
	if opts.Rebase {
		args = append(args, "--rebase")
	}
	if opts.FFOnly {
		args = append(args, "--ff-only")
	}
	if opts.Remote != "" {
		args = append(args, opts.Remote)
		if opts.Branch != "" {
			args = append(args, opts.Branch)
		}
	} else if opts.Branch != "" {
		args = append(args, "origin", opts.Branch)
	}
	_, err := run(ctx, r.Dir, args, shell.Options{})
	return err
}

// Fetch runs git fetch.
func (r *Repo) Fetch(ctx context.Context, opts FetchOptions) error {
	args := []string{"fetch"}
	if opts.Prune {
		args = append(args, "--prune")
	}
	if opts.Tags {
		args = append(args, "--tags")
	}
	if opts.Depth > 0 {
		args = append(args, fmt.Sprintf("--depth=%d", opts.Depth))
	}
	remote := opts.Remote
	if remote == "" {
		remote = "origin"
	}
	args = append(args, remote)
	_, err := run(ctx, r.Dir, args, shell.Options{})
	return err
}

// Checkout checks out a branch or commit.
func (r *Repo) Checkout(ctx context.Context, ref string, opts CheckoutOptions) error {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return fmt.Errorf("git: empty ref")
	}
	args := []string{"checkout"}
	if opts.Force {
		args = append(args, "--force")
	}
	if opts.CreateBranch {
		args = append(args, "-b", ref)
	} else {
		args = append(args, ref)
	}
	_, err := run(ctx, r.Dir, args, shell.Options{})
	return err
}

// Add stages paths (empty means all).
func (r *Repo) Add(ctx context.Context, paths ...string) error {
	args := []string{"add"}
	if len(paths) == 0 {
		args = append(args, "-A")
	} else {
		args = append(args, paths...)
	}
	_, err := run(ctx, r.Dir, args, shell.Options{})
	return err
}

// Commit creates a commit with the given message.
func (r *Repo) Commit(ctx context.Context, message string, opts CommitOptions) (string, error) {
	message = strings.TrimSpace(message)
	if message == "" {
		return "", fmt.Errorf("git: empty commit message")
	}
	args := []string{"commit", "-m", message}
	if opts.AllowEmpty {
		args = append(args, "--allow-empty")
	}
	if opts.All {
		args = append(args, "-a")
	}
	out, err := run(ctx, r.Dir, args, shell.Options{})
	if err != nil {
		return "", err
	}
	return parseCommitHash(out.Stdout), nil
}

// Reset runs git reset with the given mode (mixed, soft, hard, merge) and optional ref.
func (r *Repo) Reset(ctx context.Context, mode, ref string) error {
	mode = strings.TrimSpace(strings.ToLower(mode))
	if mode == "" {
		mode = "mixed"
	}
	switch mode {
	case "soft", "mixed", "hard", "merge":
	default:
		return fmt.Errorf("git: unknown reset mode %q", mode)
	}
	args := []string{"reset", "--" + mode}
	if ref != "" {
		args = append(args, ref)
	}
	_, err := run(ctx, r.Dir, args, shell.Options{})
	return err
}

// Clean removes untracked files. When removeIgnored is true, ignored files are removed too.
func (r *Repo) Clean(ctx context.Context, removeIgnored bool) error {
	args := []string{"clean", "-fd"}
	if removeIgnored {
		args = []string{"clean", "-ffd"}
	}
	_, err := run(ctx, r.Dir, args, shell.Options{})
	return err
}

func parseCommitHash(stdout string) string {
	for _, line := range lines(stdout) {
		if strings.HasPrefix(line, "[") {
			// [main abc1234] message
			parts := strings.Fields(strings.Trim(line, "[]"))
			for _, p := range parts {
				if len(p) >= 7 && !strings.Contains(p, "/") {
					return p
				}
			}
		}
	}
	return ""
}
