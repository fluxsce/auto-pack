package projectrelease

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"auto-pack/backend/internal/repository"
	"auto-pack/backend/pkg/git"
	"auto-pack/backend/pkg/redact"
)

// EnsureGitRepository clones or initializes git in workDir when it is not yet a repository.
func (g *GitCommitter) EnsureGitRepository(
	ctx context.Context,
	project *repository.PackProject,
	workDir string,
	releaseBranch string,
	log *publishLogger,
) error {
	if git.IsRepository(ctx, workDir) {
		return nil
	}
	cloneURL, err := g.authenticatedRepoURL(project)
	if err != nil {
		return err
	}
	shellEnv, err := g.shellEnv(project)
	if err != nil {
		return err
	}
	if err := ensureProjectParentDir(workDir); err != nil {
		return err
	}
	releaseBranch = strings.TrimSpace(releaseBranch)
	empty, err := isWorkDirEmpty(workDir)
	if err != nil {
		return err
	}
	if empty {
		cloneBranch := pickCloneBranch(ctx, cloneURL, shellEnv, releaseBranch, project.RepoBranch, log)
		log.step("工作区非 Git 仓库，正在克隆 %s", redact.URL(cloneURL))
		if err := cloneRepository(ctx, cloneURL, workDir, cloneBranch, shellEnv, log); err != nil {
			return fmt.Errorf("git clone: %w", err)
		}
		log.step("克隆完成")
		return nil
	}
	log.step("工作区非 Git 仓库且目录非空，初始化 Git 并关联远程 %s", redact.URL(cloneURL))
	if err := initRepositoryFromRemote(ctx, workDir, cloneURL, shellEnv, log); err != nil {
		return fmt.Errorf("git init: %w", err)
	}
	return nil
}

func pickCloneBranch(
	ctx context.Context,
	cloneURL string,
	env []string,
	releaseBranch string,
	repoBranch string,
	log *publishLogger,
) string {
	if releaseBranch != "" {
		onOrigin, err := remoteHeadBranchExists(ctx, cloneURL, env, releaseBranch)
		if err == nil && onOrigin {
			log.step("远程存在发版分支 %s，克隆时检出该分支", releaseBranch)
			return releaseBranch
		}
	}
	repoBranch = strings.TrimSpace(repoBranch)
	if git.IsTagRef(repoBranch) {
		tagName := git.NormalizeTagName(repoBranch)
		if tagName != "" {
			onOrigin, err := remoteTagRefExists(ctx, cloneURL, env, tagName)
			if err == nil && onOrigin {
				log.step("远程存在 Tag %s，完整克隆后检出发版分支", tagName)
				return ""
			}
		}
	} else if repoBranch != "" {
		onOrigin, err := remoteHeadBranchExists(ctx, cloneURL, env, repoBranch)
		if err == nil && onOrigin {
			log.step("远程存在工程分支 %s，克隆时检出该分支", repoBranch)
			return repoBranch
		}
	}
	log.step("远程无匹配分支，执行完整克隆后在发版流程中检出发版分支")
	return ""
}

func remoteTagRefExists(ctx context.Context, repoURL string, env []string, tagName string) (bool, error) {
	tagName = git.NormalizeTagName(tagName)
	if tagName == "" {
		return false, nil
	}
	res, err := runGitAllow(ctx, "", env, "ls-remote", "--tags", repoURL, "refs/tags/"+tagName)
	if err != nil {
		return false, err
	}
	if res.ExitCode != 0 {
		return false, nil
	}
	return strings.TrimSpace(res.Stdout) != "", nil
}

func remoteHeadBranchExists(ctx context.Context, repoURL string, env []string, branch string) (bool, error) {
	repoURL = strings.TrimSpace(repoURL)
	branch = strings.TrimSpace(branch)
	if repoURL == "" || branch == "" {
		return false, nil
	}
	res, err := runGitAllow(ctx, "", env, "ls-remote", "--heads", repoURL, branch)
	if err != nil {
		return false, err
	}
	if res.ExitCode != 0 {
		return false, nil
	}
	return strings.TrimSpace(res.Stdout) != "", nil
}

func cloneRepository(ctx context.Context, cloneURL, workDir, branch string, env []string, log *publishLogger) error {
	parent := filepath.Dir(workDir)
	if parent == "" || parent == "." {
		return fmt.Errorf("invalid clone target parent for %s", workDir)
	}
	args := []string{"clone", "--progress"}
	if branch != "" {
		args = append(args, "-b", branch, "--single-branch")
	}
	args = append(args, cloneURL, filepath.Base(workDir))
	if _, err := runGit(ctx, parent, env, args...); err != nil {
		return err
	}
	return nil
}

func initRepositoryFromRemote(ctx context.Context, workDir, cloneURL string, env []string, log *publishLogger) error {
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		return err
	}
	if _, err := runGit(ctx, workDir, env, "init"); err != nil {
		return err
	}
	_, _ = runGitAllow(ctx, workDir, env, "remote", "remove", "origin")
	if _, err := runGit(ctx, workDir, env, "remote", "add", "origin", cloneURL); err != nil {
		return err
	}
	return syncOriginRefs(ctx, workDir, env, log)
}

func ensureProjectParentDir(workDir string) error {
	parent := filepath.Dir(workDir)
	if parent == "" || parent == "." {
		return nil
	}
	return os.MkdirAll(parent, 0o755)
}

func isWorkDirEmpty(dir string) (bool, error) {
	info, err := os.Stat(dir)
	if os.IsNotExist(err) {
		return true, nil
	}
	if err != nil {
		return false, err
	}
	if !info.IsDir() {
		return false, fmt.Errorf("%s is not a directory", dir)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false, err
	}
	return len(entries) == 0, nil
}
