package projectrelease

import (
	"context"
	"fmt"
	"strings"
)

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

// originHeadBranchExists reports whether origin has a branch tip (via ls-remote).
func originHeadBranchExists(ctx context.Context, dir string, env []string, branch string) (bool, error) {
	branch = strings.TrimSpace(branch)
	if branch == "" {
		return false, nil
	}
	res, err := runGitAllow(ctx, dir, env, "ls-remote", "--heads", "origin", branch)
	if err != nil {
		return false, err
	}
	if res.ExitCode != 0 {
		return false, nil
	}
	return strings.TrimSpace(res.Stdout) != "", nil
}

func fetchOriginBranch(ctx context.Context, dir string, env []string, branch string, log *publishLogger) error {
	branch = strings.TrimSpace(branch)
	if branch == "" {
		return fmt.Errorf("branch is empty")
	}
	refSpec := "refs/heads/" + branch + ":" + originRemoteBranchRef(branch)
	log.step("git fetch origin %s", refSpec)
	if _, err := runGit(ctx, dir, env, "fetch", "origin", refSpec); err != nil {
		log.step("git fetch origin %s（简写）", branch)
		if _, err2 := runGit(ctx, dir, env, "fetch", "origin", branch); err2 != nil {
			return err2
		}
	}
	return nil
}

// syncOriginRefs fetches remote branches/tags and prunes deleted remote-tracking refs.
func syncOriginRefs(ctx context.Context, dir string, env []string, log *publishLogger) error {
	log.step("git fetch --prune --tags origin")
	if _, err := runGit(ctx, dir, env, "fetch", "--prune", "--tags", "--progress", "origin"); err != nil {
		return err
	}
	return nil
}

func checkoutFromOriginBranch(ctx context.Context, dir string, env []string, releaseBranch string, log *publishLogger) error {
	ref := originBranchRef(releaseBranch)
	log.step("从远程 %s 检出发版分支 %s", ref, releaseBranch)
	if _, err := runGit(ctx, dir, env, "checkout", "-f", "-B", releaseBranch, ref); err != nil {
		return fmt.Errorf("checkout from %s: %w", ref, err)
	}
	_, _ = runGitAllow(ctx, dir, env, "reset", "--hard", ref)
	return nil
}

func checkoutReleaseBranchFromTag(ctx context.Context, dir string, env []string, releaseBranch, versionTag string, log *publishLogger) error {
	gitTag := gitTagNameFromReleaseBranch(releaseBranch)
	if gitTag == "" {
		return fmt.Errorf(
			"cannot create branch %q: release branch name must be like release/V9.0.0-P01 to map to a git tag",
			releaseBranch,
		)
	}
	log.step("远程无发版分支 %s，从 Git 标签 %s 创建（不使用本地缓存分支）", releaseBranch, gitTag)
	ensureGitTagFetched(ctx, dir, env, gitTag)

	tagRefs := resolveGitTagRefsForCheckout(releaseBranch, versionTag)
	var tried []string
	for _, ref := range tagRefs {
		tried = append(tried, ref)
		if !gitRefExists(ctx, dir, env, ref) {
			continue
		}
		log.step("git checkout -f -B %s %s", releaseBranch, ref)
		if err := checkoutNewBranchFromRef(ctx, dir, env, releaseBranch, ref); err == nil {
			log.step("已从 %s 创建发版分支 %s", ref, releaseBranch)
			return nil
		}
	}
	return fmt.Errorf(
		"cannot create branch %q from git tag %q (tried: %s; ensure tag exists: git fetch --tags origin)",
		releaseBranch, gitTag, strings.Join(tried, ", "),
	)
}

// checkoutReleaseBranch prefers origin over stale local branches.
func checkoutReleaseBranch(ctx context.Context, dir string, env []string, releaseBranch, versionTag string, log *publishLogger) error {
	releaseBranch = strings.TrimSpace(releaseBranch)
	if releaseBranch == "" {
		return fmt.Errorf("release branch is required")
	}
	originRef := originBranchRef(releaseBranch)
	if gitRefExists(ctx, dir, env, originRef) {
		return checkoutFromOriginBranch(ctx, dir, env, releaseBranch, log)
	}
	onOrigin, err := originHeadBranchExists(ctx, dir, env, releaseBranch)
	if err != nil {
		return fmt.Errorf("ls-remote origin %s: %w", releaseBranch, err)
	}
	if onOrigin {
		log.step("远程发版分支 %s 存在，同步到本地后检出", releaseBranch)
		if err := fetchOriginBranch(ctx, dir, env, releaseBranch, log); err != nil {
			return fmt.Errorf("fetch origin %s: %w", releaseBranch, err)
		}
		if gitRefExists(ctx, dir, env, originRef) {
			return checkoutFromOriginBranch(ctx, dir, env, releaseBranch, log)
		}
		return fmt.Errorf("fetched origin/%s but remote-tracking ref is missing", releaseBranch)
	}
	if gitRefExists(ctx, dir, env, releaseBranch) {
		log.step("远程已删除 %s，忽略本地残留分支", releaseBranch)
	}
	return checkoutReleaseBranchFromTag(ctx, dir, env, releaseBranch, versionTag, log)
}
