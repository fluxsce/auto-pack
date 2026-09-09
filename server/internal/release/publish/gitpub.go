package publish

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"auto-pack/server/internal/host/git"
	"auto-pack/server/internal/project"
	"auto-pack/server/internal/release/pom"
)

const pomFileName = "pom.xml"

// GitCommitter checks out, commits, and pushes a release using host/git.
type GitCommitter struct {
	OptFor func(prj project.Project) git.Options
}

func (g *GitCommitter) opt(prj project.Project) git.Options {
	var opt git.Options
	if g != nil && g.OptFor != nil {
		opt = g.OptFor(prj)
	}
	if strings.TrimSpace(opt.Auth.CommitName) == "" {
		opt.Auth.CommitName = "auto-pack"
	}
	if strings.TrimSpace(opt.Auth.CommitEmail) == "" {
		opt.Auth.CommitEmail = "auto-pack@local"
	}
	return opt
}

// SyncReleaseWorktree fetches origin and checks out the release branch (or creates it from the base git tag).
func (g *GitCommitter) SyncReleaseWorktree(ctx context.Context, prj project.Project, workDir, releaseBranch, versionTag string, log *Logger) error {
	releaseBranch = strings.TrimSpace(releaseBranch)
	versionTag = strings.TrimSpace(versionTag)
	if releaseBranch == "" {
		return fmt.Errorf("release branch is required")
	}
	if versionTag == "" {
		return fmt.Errorf("version tag is required")
	}
	opt := g.opt(prj)
	if err := ensureGitRepository(ctx, prj, workDir, releaseBranch, opt, log); err != nil {
		return err
	}
	if err := syncOriginRefs(ctx, workDir, opt, log); err != nil {
		return fmt.Errorf("git fetch: %w", err)
	}
	log.Step("同步发版分支工作区 %s", releaseBranch)
	return checkoutReleaseBranch(ctx, workDir, opt, releaseBranch, versionTag, log)
}

// PublishChanges syncs the release branch, writes files, commits, and pushes.
func (g *GitCommitter) PublishChanges(
	ctx context.Context,
	prj project.Project,
	workDir, releaseBranch, tagRef, targetBranch string,
	createTargetBranch bool,
	commitMsg string,
	allowRepeat bool,
	files map[string][]byte,
	steps *[]string,
) error {
	log := NewLogger(steps)
	name := strings.TrimSpace(prj.ProjectName)
	log.Step("Git 发版开始（工程 %s）", name)
	repoURL := strings.TrimSpace(prj.RepoURL)
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
	log.Step("Git 提交说明: %s", strings.TrimSpace(commitMsg))
	opt := g.opt(prj)
	if err := ensureGitRepository(ctx, prj, workDir, releaseBranch, opt, log); err != nil {
		return err
	}
	if err := syncOriginRefs(ctx, workDir, opt, log); err != nil {
		log.Step("git fetch 失败")
		return fmt.Errorf("git fetch: %w", err)
	}
	log.Step("确认发版分支 %s", releaseBranch)
	if err := checkoutReleaseBranch(ctx, workDir, opt, releaseBranch, tagRef, log); err != nil {
		return err
	}

	targetBranch = strings.TrimSpace(targetBranch)
	deferPom := DeferPomToTargetBranch(createTargetBranch, releaseBranch, targetBranch)
	releaseFiles, pomFiles := splitReleasePublishFiles(files)

	if !deferPom {
		if err := writeCommitPushBranch(ctx, workDir, opt, releaseBranch, files, tagRef, commitMsg, allowRepeat, log); err != nil {
			return err
		}
		if createTargetBranch && targetBranch != "" && !strings.EqualFold(targetBranch, releaseBranch) {
			log.Step("同步推送到目标分支 %s", targetBranch)
			if err := pushBranchFromHEAD(ctx, workDir, opt, targetBranch, log); err != nil {
				return fmt.Errorf("git push target branch %s: %w", targetBranch, err)
			}
		}
		return createAndPushVersionTag(ctx, workDir, opt, tagRef, allowRepeat, log)
	}

	log.Step("已开启目标分支：pom.xml 仅在目标分支 %s 更新，发版分支 %s 不写 POM", targetBranch, releaseBranch)
	if err := writeCommitPushBranch(ctx, workDir, opt, releaseBranch, releaseFiles, tagRef, commitMsg, allowRepeat, log); err != nil {
		return err
	}
	if len(pomFiles) > 0 {
		log.Step("检出目标分支并写入 POM: %s", targetBranch)
		if err := checkoutOrCreateBranchAtHEAD(ctx, workDir, opt, targetBranch, log); err != nil {
			return fmt.Errorf("checkout target branch %s: %w", targetBranch, err)
		}
		if err := writeCommitPushBranch(ctx, workDir, opt, targetBranch, pomFiles, tagRef, commitMsg, true, log); err != nil {
			return err
		}
	} else {
		log.Step("同步推送到目标分支 %s（无 POM 变更）", targetBranch)
		if err := pushBranchFromHEAD(ctx, workDir, opt, targetBranch, log); err != nil {
			return fmt.Errorf("git push target branch %s: %w", targetBranch, err)
		}
	}
	return createAndPushVersionTag(ctx, workDir, opt, tagRef, allowRepeat, log)
}

// DeferPomToTargetBranch reports whether POM updates should be committed only on the target branch.
func DeferPomToTargetBranch(createTarget bool, releaseBranch, targetBranch string) bool {
	if !createTarget {
		return false
	}
	targetBranch = strings.TrimSpace(targetBranch)
	releaseBranch = strings.TrimSpace(releaseBranch)
	return targetBranch != "" && !strings.EqualFold(targetBranch, releaseBranch)
}

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
	opt git.Options,
	branch string,
	files map[string][]byte,
	tagRef, commitMsg string,
	allowRepeat bool,
	log *Logger,
) error {
	branch = strings.TrimSpace(branch)
	if branch == "" {
		return fmt.Errorf("branch is empty")
	}
	if len(files) == 0 {
		log.Step("分支 %s 无待写入发版文件，跳过提交", branch)
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
	log.Step("写入发版文件到 %s: %s", branch, strings.Join(relPaths, ", "))
	for path, data := range files {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return fmt.Errorf("mkdir %s: %w", filepath.Dir(path), err)
		}
		if err := os.WriteFile(path, data, 0o644); err != nil {
			return fmt.Errorf("write %s: %w", path, err)
		}
	}
	if err := verifyWrittenReleaseFiles(files, tagRef, log); err != nil {
		return err
	}
	addArgs := append([]string{"add", "-f"}, relPaths...)
	if _, err := runGit(ctx, workDir, opt, addArgs...); err != nil {
		return fmt.Errorf("git add: %w", err)
	}
	if res, err := runGitAllow(ctx, workDir, opt, "diff", "--cached", "--name-status"); err == nil {
		stat := strings.TrimSpace(res.Stdout)
		if stat == "" {
			log.Step("暂存区无差异（发版文件与 HEAD 一致，pom/Version 可能未被提交进新变更）")
		} else {
			log.Step("暂存变更:\n%s", stat)
		}
	}
	if err := gitCommitRelease(ctx, workDir, opt, commitMsg, log); err != nil {
		log.Step("git commit 失败: %v", err)
		return err
	}
	return pushBranchWithRepeatFallback(ctx, workDir, opt, branch, allowRepeat, log)
}

func verifyWrittenReleaseFiles(files map[string][]byte, tagRef string, log *Logger) error {
	wantPom := pom.PomVersionForTag(tagRef)
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
			log.Step("已校验磁盘 %s 含版本 %s", base, wantPom)
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
	return strings.ReplaceAll(rel, "\\", "/"), nil
}

func ensureGitRepository(ctx context.Context, prj project.Project, workDir, branch string, opt git.Options, log *Logger) error {
	if git.IsRepo(workDir) {
		return nil
	}
	url := strings.TrimSpace(prj.RepoURL)
	if url == "" {
		return fmt.Errorf("project has no repository URL configured")
	}
	log.Step("检出目录不是 Git 仓库，从 %s 克隆", url)
	return git.Clone(ctx, url, workDir, branch, opt, nil)
}
