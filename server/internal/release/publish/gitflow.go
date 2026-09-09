package publish

import (
	"context"
	"fmt"
	"strings"

	"auto-pack/server/internal/host/git"
)

func syncOriginRefs(ctx context.Context, dir string, opt git.Options, log *Logger) error {
	args := fetchDepthArgs(opt, []string{"fetch", "--prune", "--progress", "origin"})
	log.Step("git %s", strings.Join(args, " "))
	_, err := runGit(ctx, dir, opt, args...)
	return err
}

func fetchOriginBranch(ctx context.Context, dir string, opt git.Options, branch string, log *Logger) error {
	branch = strings.TrimSpace(branch)
	if branch == "" {
		return fmt.Errorf("branch is empty")
	}
	refSpec := "refs/heads/" + branch + ":" + originRemoteBranchRef(branch)
	args := fetchDepthArgs(opt, []string{"fetch", "origin", refSpec})
	log.Step("git %s", strings.Join(args, " "))
	if _, err := runGit(ctx, dir, opt, args...); err != nil {
		fallback := fetchDepthArgs(opt, []string{"fetch", "origin", branch})
		log.Step("git fetch origin %s（简写）", branch)
		if _, err2 := runGit(ctx, dir, opt, fallback...); err2 != nil {
			return err2
		}
	}
	return nil
}

func checkoutFromOriginBranch(ctx context.Context, dir string, opt git.Options, releaseBranch string, log *Logger) error {
	ref := originBranchRef(releaseBranch)
	log.Step("从远程 %s 强制检出发版分支 %s", ref, releaseBranch)
	if _, err := runGit(ctx, dir, opt, "checkout", "-f", "-B", releaseBranch, ref); err != nil {
		return fmt.Errorf("checkout from %s: %w", ref, err)
	}
	if _, err := runGit(ctx, dir, opt, "reset", "--hard", ref); err != nil {
		return fmt.Errorf("reset --hard %s: %w", ref, err)
	}
	log.Step("工作区已与 %s 对齐", ref)
	return nil
}

func ensureGitTagFetched(ctx context.Context, dir string, opt git.Options, tagName string) {
	tagName = normalizeGitTagName(tagName)
	if tagName == "" {
		return
	}
	refSpec := "+refs/tags/" + tagName + ":refs/tags/" + tagName
	_, _ = runGitAllow(ctx, dir, opt, "fetch", "origin", refSpec)
}

func checkoutNewBranchFromRef(ctx context.Context, dir string, opt git.Options, branch, ref string) error {
	if _, err := runGit(ctx, dir, opt, "checkout", "-f", "-B", branch, ref); err == nil {
		return nil
	}
	if _, err := runGit(ctx, dir, opt, "checkout", "-f", "-b", branch, ref); err == nil {
		return nil
	}
	return fmt.Errorf("checkout -f -B %s %s failed", branch, ref)
}

func checkoutReleaseBranchFromTag(ctx context.Context, dir string, opt git.Options, releaseBranch, versionTag string, log *Logger) error {
	gitTag := gitTagNameFromReleaseBranch(releaseBranch)
	if gitTag == "" {
		return fmt.Errorf(
			"cannot create branch %q: release branch name must be like release/V9.0.0-P01 to map to a git tag",
			releaseBranch,
		)
	}
	log.Step("远程无发版分支 %s，从 Git 标签 %s 创建（不使用本地缓存分支）", releaseBranch, gitTag)
	ensureGitTagFetched(ctx, dir, opt, gitTag)
	tagRefs := resolveGitTagRefsForCheckout(releaseBranch, versionTag)
	var tried []string
	for _, ref := range tagRefs {
		tried = append(tried, ref)
		if !gitRefExists(ctx, dir, opt, ref) {
			continue
		}
		log.Step("git checkout -f -B %s %s", releaseBranch, ref)
		if err := checkoutNewBranchFromRef(ctx, dir, opt, releaseBranch, ref); err == nil {
			log.Step("已从 %s 创建发版分支 %s", ref, releaseBranch)
			return nil
		}
	}
	return fmt.Errorf(
		"cannot create branch %q from git tag %q (tried: %s; ensure tag exists: git fetch --tags origin)",
		releaseBranch, gitTag, strings.Join(tried, ", "),
	)
}

func checkoutReleaseBranch(ctx context.Context, dir string, opt git.Options, releaseBranch, versionTag string, log *Logger) error {
	releaseBranch = strings.TrimSpace(releaseBranch)
	if releaseBranch == "" {
		return fmt.Errorf("release branch is required")
	}
	onOrigin, err := originHeadBranchExists(ctx, dir, opt, releaseBranch)
	if err != nil {
		return fmt.Errorf("ls-remote origin %s: %w", releaseBranch, err)
	}
	if onOrigin {
		log.Step("远程发版分支 %s 存在，强制拉取并检出（忽略本地分支）", releaseBranch)
		if err := fetchOriginBranch(ctx, dir, opt, releaseBranch, log); err != nil {
			return fmt.Errorf("fetch origin %s: %w", releaseBranch, err)
		}
		originRef := originBranchRef(releaseBranch)
		if !gitRefExists(ctx, dir, opt, originRef) {
			return fmt.Errorf("fetched origin/%s but remote-tracking ref is missing", releaseBranch)
		}
		return checkoutFromOriginBranch(ctx, dir, opt, releaseBranch, log)
	}
	if gitRefExists(ctx, dir, opt, releaseBranch) {
		log.Step("远程无发版分支 %s，忽略本地残留分支", releaseBranch)
	}
	return checkoutReleaseBranchFromTag(ctx, dir, opt, releaseBranch, versionTag, log)
}

func checkoutOrCreateBranchAtHEAD(ctx context.Context, dir string, opt git.Options, branch string, log *Logger) error {
	branch = strings.TrimSpace(branch)
	if branch == "" {
		return fmt.Errorf("branch is empty")
	}
	log.Step("git checkout -f -B %s", branch)
	_, err := runGit(ctx, dir, opt, "checkout", "-f", "-B", branch)
	return err
}

func hasStagedChanges(ctx context.Context, dir string, opt git.Options) bool {
	res, err := runGitAllow(ctx, dir, opt, "diff", "--cached", "--quiet")
	return err == nil && res.ExitCode == 1
}

func gitCommitRelease(ctx context.Context, dir string, opt git.Options, commitMsg string, log *Logger) error {
	commitMsg = strings.TrimSpace(commitMsg)
	if commitMsg == "" {
		commitMsg = "auto-pack release"
	}
	if hasStagedChanges(ctx, dir, opt) {
		log.Step("git commit（含发版文件变更）")
		res, err := runGitAllow(ctx, dir, opt, "-c", "commit.gpgsign=false", "commit", "-m", commitMsg)
		if err == nil && res.ExitCode == 0 {
			return nil
		}
		log.Step("常规提交未成功（exit %d），改用 git commit --allow-empty", res.ExitCode)
	} else {
		log.Step("发版文件与当前分支一致，git commit --allow-empty")
	}
	_, err := runGit(ctx, dir, opt, "-c", "commit.gpgsign=false", "commit", "--allow-empty", "-m", commitMsg)
	return err
}

func createAndPushVersionTag(ctx context.Context, dir string, opt git.Options, versionTag string, allowRepeat bool, log *Logger) error {
	versionTag = normalizeGitTagName(versionTag)
	if versionTag == "" {
		return fmt.Errorf("version tag is empty")
	}
	if allowRepeat {
		log.Step("git tag -f %s（覆盖本地标签）", versionTag)
		if _, err := runGit(ctx, dir, opt, "tag", "-f", versionTag); err != nil {
			return fmt.Errorf("git tag -f %s: %w", versionTag, err)
		}
	} else {
		if gitRefExists(ctx, dir, opt, "refs/tags/"+versionTag) {
			return fmt.Errorf("git tag %q already exists", versionTag)
		}
		log.Step("git tag %s", versionTag)
		if _, err := runGit(ctx, dir, opt, "tag", versionTag); err != nil {
			return fmt.Errorf("git tag %s: %w", versionTag, err)
		}
	}
	if !gitRefExists(ctx, dir, opt, "refs/tags/"+versionTag) {
		return fmt.Errorf("git tag %q was not created locally", versionTag)
	}
	log.Step("git push origin refs/tags/%s", versionTag)
	if err := pushVersionTagToOrigin(ctx, dir, opt, versionTag, allowRepeat); err != nil {
		return fmt.Errorf("git push tag %s: %w", versionTag, err)
	}
	log.Step("已创建并推送版本 Tag %s", versionTag)
	return nil
}

func pushVersionTagToOrigin(ctx context.Context, dir string, opt git.Options, versionTag string, allowRepeat bool) error {
	refSpec := "refs/tags/" + versionTag + ":refs/tags/" + versionTag
	var attempts [][]string
	if allowRepeat {
		attempts = [][]string{
			{"push", "-f", "origin", refSpec},
			{"push", "-f", "origin", "tag", versionTag},
			{"push", "-f", "origin", versionTag},
		}
	} else {
		attempts = [][]string{
			{"push", "origin", refSpec},
			{"push", "origin", "tag", versionTag},
			{"push", "origin", versionTag},
		}
	}
	var lastErr error
	for _, args := range attempts {
		if _, err := runGit(ctx, dir, opt, args...); err == nil {
			return nil
		} else {
			lastErr = err
		}
	}
	if lastErr != nil {
		return lastErr
	}
	return fmt.Errorf("push version tag %q failed", versionTag)
}

func pushBranchWithRepeatFallback(ctx context.Context, dir string, opt git.Options, branch string, allowRepeat bool, log *Logger) error {
	branch = strings.TrimSpace(branch)
	if branch == "" {
		return fmt.Errorf("branch is empty")
	}
	log.Step("git push origin %s", branch)
	if _, err := runGit(ctx, dir, opt, "push", "-u", "origin", branch); err == nil {
		return nil
	} else if !allowRepeat {
		return fmt.Errorf("git push %s: %w", branch, err)
	}
	if exists, err := originHeadBranchExists(ctx, dir, opt, branch); err == nil && exists {
		if fetchErr := fetchOriginBranch(ctx, dir, opt, branch, log); fetchErr != nil {
			log.Step("刷新远程跟踪分支 origin/%s 失败（继续 force-with-lease）: %v", branch, fetchErr)
		}
	}
	log.Step("git push --force-with-lease origin %s（重复发版）", branch)
	_, err2 := runGit(ctx, dir, opt, "push", "-u", "--force-with-lease", "origin", branch)
	if err2 == nil {
		return nil
	}
	if isUnknownGitPushOption(err2, "force-with-lease") || isStaleForceWithLease(err2) {
		reason := "旧版 Git，无 force-with-lease"
		if isStaleForceWithLease(err2) {
			reason = "force-with-lease stale info，改用 --force"
		}
		log.Step("git push --force origin %s（%s）", branch, reason)
		if _, err3 := runGit(ctx, dir, opt, "push", "-u", "--force", "origin", branch); err3 != nil {
			return fmt.Errorf("git push %s: %w", branch, err3)
		}
		return nil
	}
	return fmt.Errorf("git push %s: %w", branch, err2)
}

func pushBranchFromHEAD(ctx context.Context, dir string, opt git.Options, targetBranch string, log *Logger) error {
	targetBranch = strings.TrimSpace(targetBranch)
	if targetBranch == "" {
		return fmt.Errorf("target branch is empty")
	}
	if cur := currentGitBranch(ctx, dir, opt); !strings.EqualFold(cur, targetBranch) {
		log.Step("git branch -f %s HEAD", targetBranch)
		if _, err := runGit(ctx, dir, opt, "branch", "-f", targetBranch, "HEAD"); err != nil {
			return err
		}
	}
	if err := pushBranchWithRepeatFallback(ctx, dir, opt, targetBranch, true, log); err != nil {
		return err
	}
	log.Step("已推送到目标分支 %s", targetBranch)
	return nil
}

func isUnknownGitPushOption(err error, option string) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	opt := strings.ToLower(strings.TrimSpace(option))
	return strings.Contains(msg, "unknown option") && strings.Contains(msg, opt)
}

func isStaleForceWithLease(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "stale info") ||
		(strings.Contains(msg, "rejected") && strings.Contains(msg, "force-with-lease"))
}

// ListRemoteTags returns origin tag names from git ls-remote --tags.
func ListRemoteTags(ctx context.Context, dir string, opt git.Options) []string {
	res, err := runGitAllow(ctx, dir, opt, "ls-remote", "--tags", "origin")
	if err != nil || res.ExitCode != 0 {
		return nil
	}
	var tags []string
	for _, line := range strings.Split(res.Stdout, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		ref := fields[1]
		if strings.HasSuffix(ref, "^{}") {
			continue
		}
		tags = append(tags, normalizeGitTagName(ref))
	}
	return tags
}
