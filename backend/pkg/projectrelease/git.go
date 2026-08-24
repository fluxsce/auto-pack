package projectrelease

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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
	if err := g.EnsureGitRepository(ctx, project, workDir, releaseBranch, log); err != nil {
		return err
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
// Version.txt (and other non-pom files) stay on the release branch; pom.xml updates
// are committed only on the target branch (e.g. develop → release/V9.0.0-P08).
// Without a distinct target branch, all files (including POM) are written on the release branch.
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

	if err := g.EnsureGitRepository(ctx, project, workDir, releaseBranch, log); err != nil {
		return err
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

	targetBranch = strings.TrimSpace(targetBranch)
	deferPom := deferPomToTargetBranch(createTargetBranch, releaseBranch, targetBranch)
	releaseFiles, pomFiles := splitReleasePublishFiles(files)

	if !deferPom {
		if err := writeCommitPushBranch(ctx, workDir, shellEnv, releaseBranch, files, tagRef, commitMsg, allowRepeat, log); err != nil {
			return err
		}
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

	log.step("已开启目标分支：pom.xml 仅在目标分支 %s 更新，发版分支 %s 不写 POM", targetBranch, releaseBranch)
	if err := writeCommitPushBranch(ctx, workDir, shellEnv, releaseBranch, releaseFiles, tagRef, commitMsg, allowRepeat, log); err != nil {
		return err
	}
	if len(pomFiles) > 0 {
		log.step("检出目标分支并写入 POM: %s", targetBranch)
		if err := checkoutOrCreateBranchAtHEAD(ctx, workDir, shellEnv, targetBranch, log); err != nil {
			return fmt.Errorf("checkout target branch %s: %w", targetBranch, err)
		}
		if err := writeCommitPushBranch(ctx, workDir, shellEnv, targetBranch, pomFiles, tagRef, commitMsg, true, log); err != nil {
			return err
		}
	} else {
		log.step("同步推送到目标分支 %s（无 POM 变更）", targetBranch)
		if err := pushBranchFromHEAD(ctx, workDir, shellEnv, targetBranch, log); err != nil {
			return fmt.Errorf("git push target branch %s: %w", targetBranch, err)
		}
	}
	if err := createAndPushVersionTag(ctx, workDir, shellEnv, tagRef, allowRepeat, log); err != nil {
		return err
	}
	return nil
}

// deferPomToTargetBranch reports whether pom.xml should be committed only on the target branch.
func deferPomToTargetBranch(createTarget bool, releaseBranch, targetBranch string) bool {
	if !createTarget {
		return false
	}
	targetBranch = strings.TrimSpace(targetBranch)
	releaseBranch = strings.TrimSpace(releaseBranch)
	return targetBranch != "" && !strings.EqualFold(targetBranch, releaseBranch)
}

// splitReleasePublishFiles separates pom.xml updates from Version.txt and other release files.
func splitReleasePublishFiles(files map[string][]byte) (releaseFiles, pomFiles map[string][]byte) {
	releaseFiles = make(map[string][]byte)
	pomFiles = make(map[string][]byte)
	for path, data := range files {
		if isPomReleaseFile(path) {
			pomFiles[path] = data
		} else {
			releaseFiles[path] = data
		}
	}
	return releaseFiles, pomFiles
}

func isPomReleaseFile(path string) bool {
	return strings.EqualFold(filepath.Base(strings.TrimSpace(path)), pomFileName)
}

func writeCommitPushBranch(
	ctx context.Context,
	workDir string,
	shellEnv []string,
	branch string,
	files map[string][]byte,
	tagRef string,
	commitMsg string,
	allowRepeat bool,
	log *publishLogger,
) error {
	branch = strings.TrimSpace(branch)
	if branch == "" {
		return fmt.Errorf("branch is empty")
	}
	if len(files) == 0 {
		log.step("分支 %s 无待写入发版文件，跳过提交", branch)
		return nil
	}

	relPaths := make([]string, 0, len(files))
	for p := range files {
		rel, relErr := relativeToWorkDir(workDir, p)
		if relErr != nil {
			return relErr
		}
		relPaths = append(relPaths, rel)
	}
	log.step("写入发版文件到 %s: %s", branch, strings.Join(relPaths, ", "))
	for path, data := range files {
		if err := os.WriteFile(path, data, 0o644); err != nil {
			return fmt.Errorf("write %s: %w", path, err)
		}
	}
	// Fail loudly if disk does not match generated content (this is the real
	// "local pom version updated" check — earlier log lines only mean in-memory).
	if err := verifyWrittenReleaseFiles(files, tagRef, log); err != nil {
		return err
	}
	addArgs := append([]string{"add", "-f"}, relPaths...)
	if _, err := runGit(ctx, workDir, shellEnv, addArgs...); err != nil {
		return fmt.Errorf("git add: %w", err)
	}
	if res, err := runGitAllow(ctx, workDir, shellEnv, "diff", "--cached", "--name-status"); err == nil {
		stat := strings.TrimSpace(res.Stdout)
		if stat == "" {
			log.step("暂存区无差异（发版文件与 HEAD 一致，pom/Version 可能未被提交进新变更）")
		} else {
			log.step("暂存变更:\n%s", stat)
		}
	}
	if err := gitCommitRelease(ctx, workDir, shellEnv, commitMsg, log); err != nil {
		log.step("git commit 失败: %v", err)
		return err
	}
	if err := pushBranchWithRepeatFallback(ctx, workDir, shellEnv, branch, allowRepeat, log); err != nil {
		return err
	}
	return nil
}

func checkoutOrCreateBranchAtHEAD(ctx context.Context, dir string, env []string, branch string, log *publishLogger) error {
	branch = strings.TrimSpace(branch)
	if branch == "" {
		return fmt.Errorf("branch is empty")
	}
	log.step("git checkout -f -B %s", branch)
	if _, err := runGit(ctx, dir, env, "checkout", "-f", "-B", branch); err != nil {
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
	return pushBranchWithRepeatFallback(ctx, dir, env, releaseBranch, allowRepeat, log)
}

func pushBranchWithRepeatFallback(ctx context.Context, dir string, env []string, branch string, allowRepeat bool, log *publishLogger) error {
	branch = strings.TrimSpace(branch)
	if branch == "" {
		return fmt.Errorf("branch is empty")
	}
	log.step("git push origin %s", branch)
	if _, err := runGit(ctx, dir, env, "push", "-u", "origin", branch); err == nil {
		return nil
	} else if !allowRepeat {
		return fmt.Errorf("git push %s: %w", branch, err)
	}

	// Refresh remote-tracking tip before --force-with-lease. Shallow/partial fetches often
	// omit the target branch, which makes lease checks fail with "stale info".
	if exists, err := originHeadBranchExists(ctx, dir, env, branch); err == nil && exists {
		if fetchErr := fetchOriginBranch(ctx, dir, env, branch, log); fetchErr != nil {
			log.step("刷新远程跟踪分支 origin/%s 失败（继续 force-with-lease）: %v", branch, fetchErr)
		}
	}

	log.step("git push --force-with-lease origin %s（重复发版）", branch)
	_, err2 := runGit(ctx, dir, env, "push", "-u", "--force-with-lease", "origin", branch)
	if err2 == nil {
		return nil
	}
	if isUnknownGitPushOption(err2, "force-with-lease") || isStaleForceWithLease(err2) {
		reason := "旧版 Git，无 force-with-lease"
		if isStaleForceWithLease(err2) {
			reason = "force-with-lease stale info，改用 --force"
		}
		log.step("git push --force origin %s（%s）", branch, reason)
		if _, err3 := runGit(ctx, dir, env, "push", "-u", "--force", "origin", branch); err3 != nil {
			return fmt.Errorf("git push %s: %w", branch, err3)
		}
		return nil
	}
	return fmt.Errorf("git push %s: %w", branch, err2)
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

func pushBranchFromHEAD(ctx context.Context, dir string, env []string, targetBranch string, log *publishLogger) error {
	targetBranch = strings.TrimSpace(targetBranch)
	if targetBranch == "" {
		return fmt.Errorf("target branch is empty")
	}
	// branch -f cannot move the branch that is currently checked out; skip when already on target.
	if cur := currentGitBranch(ctx, dir, env); !strings.EqualFold(cur, targetBranch) {
		log.step("git branch -f %s HEAD", targetBranch)
		if _, err := runGit(ctx, dir, env, "branch", "-f", targetBranch, "HEAD"); err != nil {
			return err
		}
	}
	if err := pushBranchWithRepeatFallback(ctx, dir, env, targetBranch, true, log); err != nil {
		return err
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
	// Intentionally no --depth: release branches are often created from older
	// version tags that sit outside git.clone_depth of the current branch tip.
	refSpec := "+refs/tags/" + tagName + ":refs/tags/" + tagName
	_, _ = runGitAllow(ctx, dir, env, "fetch", "origin", refSpec)
}

func gitRefExists(ctx context.Context, dir string, env []string, ref string) bool {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return false
	}
	res, err := runGitAllow(ctx, dir, env, "rev-parse", "--verify", ref)
	return err == nil && res.ExitCode == 0
}

func currentGitBranch(ctx context.Context, dir string, env []string) string {
	res, err := runGitAllow(ctx, dir, env, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil || res.ExitCode != 0 {
		return ""
	}
	branch := strings.TrimSpace(res.Stdout)
	if branch == "HEAD" {
		return ""
	}
	return branch
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

func verifyWrittenReleaseFiles(files map[string][]byte, tagRef string, log *publishLogger) error {
	wantPom := PomVersionForTag(tagRef)
	for path, want := range files {
		got, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("re-read %s after write: %w", path, err)
		}
		if !bytes.Equal(got, want) {
			return fmt.Errorf("write verify failed for %s: on-disk content differs from generated bytes", path)
		}
		base := filepath.Base(path)
		if strings.EqualFold(base, pomFileName) {
			if wantPom != "" && !bytes.Contains(got, []byte("<version>"+wantPom+"</version>")) {
				return fmt.Errorf("write verify failed for %s: missing <version>%s</version> on disk", path, wantPom)
			}
			log.step("已校验磁盘 %s 含版本 %s", base, wantPom)
		}
	}
	return nil
}

func relativeToWorkDir(workDir, absPath string) (string, error) {
	workDir = strings.TrimSpace(workDir)
	absPath = strings.TrimSpace(absPath)
	if workDir == "" {
		return "", fmt.Errorf("empty work dir")
	}
	if absPath == "" {
		return "", fmt.Errorf("empty path")
	}
	absWork, err := filepath.Abs(workDir)
	if err != nil {
		return "", fmt.Errorf("abs work dir: %w", err)
	}
	absFile, err := filepath.Abs(absPath)
	if err != nil {
		return "", fmt.Errorf("abs path: %w", err)
	}
	rel, err := filepath.Rel(absWork, absFile)
	if err != nil {
		return "", fmt.Errorf("path %s is outside work dir %s: %w", absPath, workDir, err)
	}
	if rel == "." || rel == "" || strings.HasPrefix(rel, "..") {
		return "", fmt.Errorf("path %s is outside work dir %s", absPath, workDir)
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
