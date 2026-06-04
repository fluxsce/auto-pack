package projectrelease

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"auto-pack/backend/internal/repository"
	"auto-pack/backend/pkg/envtypes"
	"auto-pack/backend/pkg/git"
	"auto-pack/backend/pkg/shell"
)

// GitCommitter performs checkout, commit, and push for a release branch.
type GitCommitter struct {
	EnvRepo *repository.EnvRepository
}

// SyncReleaseWorktree fetches origin and checks out the release branch (or creates it from the base git tag).
// Must run before reading Version.txt / pom.xml so generation uses the release branch tree, not develop cache.
func (g *GitCommitter) SyncReleaseWorktree(
	ctx context.Context,
	project *repository.PackProject,
	workDir string,
	releaseBranch string,
	versionTag string,
	log *publishLogger,
) error {
	releaseBranch = strings.TrimSpace(releaseBranch)
	versionTag = strings.TrimSpace(versionTag)
	if releaseBranch == "" {
		return fmt.Errorf("release branch is required")
	}
	if versionTag == "" {
		return fmt.Errorf("version tag is required")
	}
	shellEnv, err := g.shellEnv(project)
	if err != nil {
		return err
	}
	if !git.IsRepository(ctx, workDir) {
		return fmt.Errorf("not a git repository: %s", workDir)
	}
	if _, err := git.Open(workDir); err != nil {
		return err
	}
	if err := syncOriginRefs(ctx, workDir, shellEnv, log); err != nil {
		return fmt.Errorf("git fetch: %w", err)
	}
	log.step("同步发版分支工作区 %s", releaseBranch)
	return checkoutReleaseBranch(ctx, workDir, shellEnv, releaseBranch, versionTag, log)
}

// PublishChanges syncs the release branch, writes files, commits, and pushes.
// When createTargetBranch is true and targetBranch differs from releaseBranch,
// the same commit is also pushed to targetBranch (e.g. release/V9.0.0-P08 from develop).
func (g *GitCommitter) PublishChanges(
	ctx context.Context,
	project *repository.PackProject,
	workDir string,
	releaseBranch string,
	tagRef string,
	targetBranch string,
	createTargetBranch bool,
	commitMsg string,
	allowRepeat bool,
	files map[string][]byte,
	steps *[]string,
) error {
	log := newPublishLogger(steps)
	name := strings.TrimSpace(project.ProjectName)
	log.step("Git 发版开始（工程 %s）", name)
	repoURL := strings.TrimSpace(project.RepoURL)
	if repoURL == "" {
		return fmt.Errorf("project has no repository URL configured")
	}
	releaseBranch = strings.TrimSpace(releaseBranch)
	if releaseBranch == "" {
		return fmt.Errorf("release branch is required")
	}
	tagRef = strings.TrimSpace(tagRef)
	if tagRef == "" {
		return fmt.Errorf("version tag is required")
	}
	log.step("Git 提交说明: %s", strings.TrimSpace(commitMsg))
	shellEnv, err := g.shellEnv(project)
	if err != nil {
		log.step("Git 环境配置失败: %v", err)
		return err
	}

	if !git.IsRepository(ctx, workDir) {
		return fmt.Errorf("not a git repository: %s", workDir)
	}
	if _, err := git.Open(workDir); err != nil {
		return err
	}

	if err := syncOriginRefs(ctx, workDir, shellEnv, log); err != nil {
		log.step("git fetch 失败")
		return fmt.Errorf("git fetch: %w", err)
	}
	log.step("确认发版分支 %s", releaseBranch)
	if err := checkoutReleaseBranch(ctx, workDir, shellEnv, releaseBranch, tagRef, log); err != nil {
		return err
	}

	for path, data := range files {
		if err := os.WriteFile(path, data, 0o644); err != nil {
			return fmt.Errorf("write %s: %w", path, err)
		}
	}

	relPaths := make([]string, 0, len(files))
	for p := range files {
		rel, relErr := relativeToWorkDir(workDir, p)
		if relErr != nil {
			return relErr
		}
		relPaths = append(relPaths, rel)
	}
	log.step("写入发版文件: %s", strings.Join(relPaths, ", "))
	addArgs := append([]string{"add", "-f"}, relPaths...)
	if _, err := runGit(ctx, workDir, shellEnv, addArgs...); err != nil {
		return fmt.Errorf("git add: %w", err)
	}
	if err := gitCommitRelease(ctx, workDir, shellEnv, commitMsg, log); err != nil {
		log.step("git commit 失败: %v", err)
		return err
	}
	if err := pushReleaseBranchToOrigin(ctx, workDir, shellEnv, releaseBranch, allowRepeat, log); err != nil {
		return err
	}

	targetBranch = strings.TrimSpace(targetBranch)
	if createTargetBranch && targetBranch != "" && !strings.EqualFold(targetBranch, releaseBranch) {
		log.step("同步推送到目标分支 %s", targetBranch)
		if err := pushBranchFromHEAD(ctx, workDir, shellEnv, targetBranch, log); err != nil {
			return fmt.Errorf("git push target branch %s: %w", targetBranch, err)
		}
	}
	if err := createAndPushVersionTag(ctx, workDir, shellEnv, tagRef, allowRepeat, log); err != nil {
		return err
	}
	return nil
}

// createAndPushVersionTag tags HEAD with the release version label and pushes to origin.
func createAndPushVersionTag(ctx context.Context, dir string, env []string, versionTag string, allowRepeat bool, log *publishLogger) error {
	versionTag = normalizeGitTagName(versionTag)
	if versionTag == "" {
		return fmt.Errorf("version tag is empty")
	}
	if allowRepeat {
		log.step("git tag -f %s（覆盖本地标签）", versionTag)
		if _, err := runGit(ctx, dir, env, "tag", "-f", versionTag); err != nil {
			return fmt.Errorf("git tag -f %s: %w", versionTag, err)
		}
	} else {
		if gitRefExists(ctx, dir, env, "refs/tags/"+versionTag) {
			return fmt.Errorf("git tag %q already exists", versionTag)
		}
		log.step("git tag %s", versionTag)
		if _, err := runGit(ctx, dir, env, "tag", versionTag); err != nil {
			return fmt.Errorf("git tag %s: %w", versionTag, err)
		}
	}
	if !gitRefExists(ctx, dir, env, "refs/tags/"+versionTag) {
		return fmt.Errorf("git tag %q was not created locally", versionTag)
	}
	log.step("git push origin refs/tags/%s", versionTag)
	if err := pushVersionTagToOrigin(ctx, dir, env, versionTag, allowRepeat); err != nil {
		return fmt.Errorf("git push tag %s: %w", versionTag, err)
	}
	log.step("已创建并推送版本 Tag %s", versionTag)
	return nil
}

func pushVersionTagToOrigin(ctx context.Context, dir string, env []string, versionTag string, allowRepeat bool) error {
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
		if _, err := runGit(ctx, dir, env, args...); err == nil {
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

// pushBranchFromHEAD creates or updates targetBranch on origin from current HEAD.
func pushReleaseBranchToOrigin(ctx context.Context, dir string, env []string, releaseBranch string, allowRepeat bool, log *publishLogger) error {
	releaseBranch = strings.TrimSpace(releaseBranch)
	if releaseBranch == "" {
		return fmt.Errorf("release branch is empty")
	}
	log.step("git push origin %s", releaseBranch)
	if _, err := runGit(ctx, dir, env, "push", "-u", "origin", releaseBranch); err == nil {
		return nil
	} else if !allowRepeat {
		return fmt.Errorf("git push %s: %w", releaseBranch, err)
	}
	log.step("git push --force-with-lease origin %s（重复发版）", releaseBranch)
	if _, err2 := runGit(ctx, dir, env, "push", "-u", "--force-with-lease", "origin", releaseBranch); err2 != nil {
		return fmt.Errorf("git push %s: %w", releaseBranch, err2)
	}
	return nil
}

func pushBranchFromHEAD(ctx context.Context, dir string, env []string, targetBranch string, log *publishLogger) error {
	targetBranch = strings.TrimSpace(targetBranch)
	if targetBranch == "" {
		return fmt.Errorf("target branch is empty")
	}
	log.step("git branch -f %s HEAD", targetBranch)
	if _, err := runGit(ctx, dir, env, "branch", "-f", targetBranch, "HEAD"); err != nil {
		return err
	}
	log.step("git push origin %s", targetBranch)
	if _, err := runGit(ctx, dir, env, "push", "-u", "origin", targetBranch); err != nil {
		if _, err2 := runGit(ctx, dir, env, "push", "-u", "--force-with-lease", "origin", targetBranch); err2 != nil {
			return err
		}
	}
	log.step("已推送到目标分支 %s", targetBranch)
	return nil
}

// checkoutNewBranchFromRef creates or resets releaseBranch at ref (-f handles build output under target/ etc.).
func checkoutNewBranchFromRef(ctx context.Context, dir string, env []string, branch, ref string) error {
	if _, err := runGit(ctx, dir, env, "checkout", "-f", "-B", branch, ref); err == nil {
		return nil
	}
	if _, err := runGit(ctx, dir, env, "checkout", "-f", "-b", branch, ref); err == nil {
		return nil
	}
	return fmt.Errorf("checkout -f -B %s %s failed", branch, ref)
}

func ensureGitTagFetched(ctx context.Context, dir string, env []string, tagName string) {
	tagName = normalizeGitTagName(tagName)
	if tagName == "" {
		return
	}
	_, _ = runGitAllow(ctx, dir, env, "fetch", "origin", "refs/tags/"+tagName+":refs/tags/"+tagName)
	_, _ = runGitAllow(ctx, dir, env, "fetch", "origin", "tag", tagName)
}

func gitRefExists(ctx context.Context, dir string, env []string, ref string) bool {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return false
	}
	res, err := runGitAllow(ctx, dir, env, "rev-parse", "--verify", ref)
	return err == nil && res.ExitCode == 0
}

func runGitAllow(ctx context.Context, dir string, env []string, args ...string) (shell.Result, error) {
	exe, err := git.Executable()
	if err != nil {
		return shell.Result{}, err
	}
	opts := shell.Options{Dir: dir, Env: env, AllowNonZeroExit: true}
	return shell.RunArgs(ctx, exe, args, opts)
}

func dedupeOrdered(items []string) []string {
	seen := make(map[string]struct{}, len(items))
	out := make([]string, 0, len(items))
	for _, s := range items {
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}

func runGit(ctx context.Context, dir string, env []string, args ...string) (shell.Result, error) {
	exe, err := git.Executable()
	if err != nil {
		return shell.Result{}, err
	}
	opts := shell.Options{Dir: dir, Env: env}
	return shell.RunArgs(ctx, exe, args, opts)
}

func relativeToWorkDir(workDir, absPath string) (string, error) {
	workDir = strings.TrimSpace(workDir)
	absPath = strings.TrimSpace(absPath)
	if absPath == "" {
		return "", fmt.Errorf("empty path")
	}
	rel := strings.TrimPrefix(absPath, workDir)
	rel = strings.TrimPrefix(rel, string(os.PathSeparator))
	rel = strings.TrimPrefix(rel, "/")
	rel = strings.TrimPrefix(rel, "\\")
	if rel == "" || rel == absPath {
		return "", fmt.Errorf("path %s is outside work dir", absPath)
	}
	return filepathSlash(rel), nil
}

func filepathSlash(p string) string {
	return strings.ReplaceAll(p, "\\", "/")
}

func (g *GitCommitter) shellEnv(project *repository.PackProject) ([]string, error) {
	shellEnv := append([]string{}, os.Environ()...)
	var cfg envtypes.GitConfig
	if project.GitEnvRecID != "" {
		item, err := g.EnvRepo.FindByRecID(project.GitEnvRecID)
		if err != nil {
			return nil, fmt.Errorf("git environment: %w", err)
		}
		if err := json.Unmarshal([]byte(item.ConfigJSON), &cfg); err != nil {
			return nil, fmt.Errorf("git environment config: %w", err)
		}
		shellEnv = applyGitIdentity(shellEnv, cfg)
		auth := strings.ToLower(strings.TrimSpace(cfg.AuthType))
		if auth == envtypes.GitAuthSSH && strings.TrimSpace(cfg.PrivateKeyPath) != "" {
			keyPath := strings.TrimSpace(cfg.PrivateKeyPath)
			sshCmd := fmt.Sprintf("ssh -i %q -o StrictHostKeyChecking=accept-new", keyPath)
			shellEnv = append(shellEnv, "GIT_SSH_COMMAND="+sshCmd)
		}
	}
	return ensureGitIdentity(shellEnv), nil
}
