package publish

import (
	"context"
	"fmt"
	"strings"

	"auto-pack/server/internal/host/git"
)

func runGit(ctx context.Context, dir string, opt git.Options, args ...string) (git.Result, error) {
	return git.ExecOK(ctx, dir, opt, args...)
}

func runGitAllow(ctx context.Context, dir string, opt git.Options, args ...string) (git.Result, error) {
	return git.Exec(ctx, dir, opt, args...)
}

func gitRefExists(ctx context.Context, dir string, opt git.Options, ref string) bool {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return false
	}
	res, err := runGitAllow(ctx, dir, opt, "rev-parse", "--verify", ref)
	return err == nil && res.ExitCode == 0
}

func currentGitBranch(ctx context.Context, dir string, opt git.Options) string {
	res, err := runGitAllow(ctx, dir, opt, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil || res.ExitCode != 0 {
		return ""
	}
	branch := strings.TrimSpace(res.Stdout)
	if branch == "HEAD" {
		return ""
	}
	return branch
}

func originHeadBranchExists(ctx context.Context, dir string, opt git.Options, branch string) (bool, error) {
	branch = strings.TrimSpace(branch)
	if branch == "" {
		return false, nil
	}
	res, err := runGitAllow(ctx, dir, opt, "ls-remote", "--heads", "origin", branch)
	if err != nil {
		return false, err
	}
	if res.ExitCode != 0 {
		return false, nil
	}
	return strings.TrimSpace(res.Stdout) != "", nil
}

func originBranchRef(branch string) string {
	return "origin/" + strings.TrimSpace(branch)
}

func originRemoteBranchRef(branch string) string {
	branch = strings.TrimSpace(branch)
	if branch == "" {
		return ""
	}
	return "refs/remotes/origin/" + branch
}

func fetchDepthArgs(opt git.Options, args []string) []string {
	if opt.Depth > 0 {
		return append(args, "--depth", fmt.Sprintf("%d", opt.Depth))
	}
	return args
}
