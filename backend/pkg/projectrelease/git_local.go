package projectrelease

import (
	"context"
	"fmt"
	"strings"

	"auto-pack/backend/internal/repository"
	apigit "auto-pack/backend/pkg/git"
	"auto-pack/backend/pkg/projectcfg"
)

// SyncLocalSourceWorktree prepares a local source directory for commit/push to origin.
func (g *GitCommitter) SyncLocalSourceWorktree(
	ctx context.Context,
	project *repository.PackProject,
	ws *repository.PackWorkspace,
	dataDir, branch string,
	log *publishLogger,
) (string, error) {
	if !projectcfg.HasRemotePush(project) {
		return "", fmt.Errorf("remote repository URL is not configured")
	}
	localDir := projectcfg.ResolveProjectSourceAbsDir(project, ws, dataDir)
	if err := projectcfg.LocalSourceDirReady(localDir); err != nil {
		return "", fmt.Errorf("local source: %w", err)
	}
	branch = strings.TrimSpace(branch)
	if branch == "" {
		branch = strings.TrimSpace(project.RepoBranch)
	}
	if branch == "" {
		branch = "main"
	}
	if apigit.IsTagRef(branch) {
		return "", fmt.Errorf("cannot push local source to tag ref %s", branch)
	}
	shellEnv, err := g.shellEnv(project)
	if err != nil {
		return "", err
	}
	log.step("使用本地源码目录 %s", localDir)
	if err := g.EnsureGitRepository(ctx, project, localDir, branch, log); err != nil {
		return "", err
	}
	if _, err := apigit.Open(localDir); err != nil {
		return "", err
	}
	if err := syncOriginRefs(ctx, localDir, shellEnv, log); err != nil {
		return "", fmt.Errorf("git fetch: %w", err)
	}
	log.step("拉取并检出分支 %s", branch)
	if err := checkoutProjectBranchRef(ctx, localDir, shellEnv, branch, log); err != nil {
		return "", err
	}
	return localDir, nil
}

// PushLocalSource commits local directory changes and pushes to origin.
func (g *GitCommitter) PushLocalSource(
	ctx context.Context,
	project *repository.PackProject,
	ws *repository.PackWorkspace,
	dataDir, commitMsg string,
	allowRepeat bool,
) ([]string, error) {
	var steps []string
	log := newPublishLogger(&steps)
	branch := strings.TrimSpace(project.RepoBranch)
	if branch == "" {
		branch = "main"
	}
	localDir, err := g.SyncLocalSourceWorktree(ctx, project, ws, dataDir, branch, log)
	if err != nil {
		return steps, err
	}
	commitMsg = strings.TrimSpace(commitMsg)
	if commitMsg == "" {
		commitMsg = "推送本地源码到远程"
	}
	shellEnv, err := g.shellEnv(project)
	if err != nil {
		return steps, err
	}
	if err := gitCommitAll(ctx, localDir, shellEnv, commitMsg, log); err != nil {
		return steps, err
	}
	if err := pushReleaseBranchToOrigin(ctx, localDir, shellEnv, branch, allowRepeat, log); err != nil {
		return steps, err
	}
	log.step("已推送本地源码到远程分支 %s", branch)
	return steps, nil
}

func gitCommitAll(ctx context.Context, dir string, env []string, message string, log *publishLogger) error {
	if _, err := runGit(ctx, dir, env, "add", "-A"); err != nil {
		return fmt.Errorf("git add: %w", err)
	}
	status, err := runGitAllow(ctx, dir, env, "status", "--porcelain")
	if err != nil {
		return err
	}
	if strings.TrimSpace(status.Stdout) == "" {
		log.step("工作区无变更，跳过 git commit")
		return nil
	}
	return gitCommitRelease(ctx, dir, env, message, log)
}

// SyncProjectSourceWorktree syncs Git for pom/release on git-source or local-source projects.
func (g *GitCommitter) SyncProjectSourceWorktree(
	ctx context.Context,
	project *repository.PackProject,
	ws *repository.PackWorkspace,
	dataDir, branch string,
	log *publishLogger,
) error {
	if projectcfg.UsesGitSource(project) {
		workDir := projectcfg.ResolveProjectSourceAbsDir(project, ws, dataDir)
		return g.SyncProjectBranchWorktree(ctx, project, workDir, branch, log)
	}
	if !projectcfg.HasRemotePush(project) {
		return fmt.Errorf("remote repository URL is not configured")
	}
	_, err := g.SyncLocalSourceWorktree(ctx, project, ws, dataDir, branch, log)
	return err
}
