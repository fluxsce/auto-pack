package projectrelease

import (
	"context"
	"fmt"
	"os"
	"strings"

	"auto-pack/backend/internal/repository"
	apigit "auto-pack/backend/pkg/git"
)

// SyncProjectBranchWorktree fetches origin and checks out the project's Git branch before pom edits.
func (g *GitCommitter) SyncProjectBranchWorktree(
	ctx context.Context,
	project *repository.PackProject,
	workDir, branch string,
	log *publishLogger,
) error {
	if strings.TrimSpace(project.RepoURL) == "" {
		return fmt.Errorf("project has no repository URL configured")
	}
	branch = strings.TrimSpace(branch)
	if branch == "" {
		branch = strings.TrimSpace(project.RepoBranch)
	}
	if branch == "" {
		branch = "main"
	}
	if apigit.IsTagRef(branch) {
		return fmt.Errorf("工程 Git 分支为 Tag（%s），无法在其上提交 POM 修改，请改为分支名", branch)
	}
	shellEnv, err := g.shellEnv(project)
	if err != nil {
		return err
	}
	if err := g.EnsureGitRepository(ctx, project, workDir, branch, log); err != nil {
		return err
	}
	if _, err := apigit.Open(workDir); err != nil {
		return err
	}
	if err := syncOriginRefs(ctx, workDir, shellEnv, log); err != nil {
		return fmt.Errorf("git fetch: %w", err)
	}
	log.step("拉取并检出工程分支 %s", branch)
	return checkoutProjectBranchRef(ctx, workDir, shellEnv, branch, log)
}

func checkoutProjectBranchRef(ctx context.Context, dir string, env []string, repoBranch string, log *publishLogger) error {
	repoBranch = strings.TrimSpace(repoBranch)
	if repoBranch == "" {
		return fmt.Errorf("repo branch is required")
	}
	if apigit.IsTagRef(repoBranch) {
		tagName := apigit.NormalizeTagName(repoBranch)
		ensureGitTagFetched(ctx, dir, env, tagName)
	}
	originRef := apigit.OriginBranchRef(repoBranch)
	if !apigit.IsTagRef(repoBranch) && gitRefExists(ctx, dir, env, originRef) {
		log.step("git checkout -f -B %s %s", repoBranch, originRef)
		if _, err := runGit(ctx, dir, env, "checkout", "-f", "-B", repoBranch, originRef); err == nil {
			return nil
		}
	}
	if !apigit.IsTagRef(repoBranch) {
		if err := fetchOriginBranch(ctx, dir, env, repoBranch, log); err == nil && gitRefExists(ctx, dir, env, originRef) {
			return checkoutFromOriginBranch(ctx, dir, env, repoBranch, log)
		}
	}
	for _, ref := range apigit.CheckoutRefCandidates(repoBranch) {
		log.step("git checkout -f %s", ref)
		if _, err := runGit(ctx, dir, env, "checkout", "-f", ref); err == nil {
			return nil
		}
	}
	return fmt.Errorf("git checkout %s failed", repoBranch)
}

// PublishBranchChanges writes files, commits, and pushes to the given branch (no version tag).
func (g *GitCommitter) PublishBranchChanges(
	ctx context.Context,
	project *repository.PackProject,
	workDir, branch, commitMsg string,
	allowRepeat bool,
	files map[string][]byte,
	steps *[]string,
) error {
	log := newPublishLogger(steps)
	branch = strings.TrimSpace(branch)
	if branch == "" {
		return fmt.Errorf("branch is required")
	}
	if apigit.IsTagRef(branch) {
		return fmt.Errorf("cannot push commit for tag ref %s", branch)
	}
	commitMsg = strings.TrimSpace(commitMsg)
	if commitMsg == "" {
		return fmt.Errorf("commit message is required")
	}
	if len(files) == 0 {
		return fmt.Errorf("no files to commit")
	}
	shellEnv, err := g.shellEnv(project)
	if err != nil {
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
	log.step("写入 POM 文件: %s", strings.Join(relPaths, ", "))
	addArgs := append([]string{"add", "-f"}, relPaths...)
	if _, err := runGit(ctx, workDir, shellEnv, addArgs...); err != nil {
		return fmt.Errorf("git add: %w", err)
	}
	if err := gitCommitRelease(ctx, workDir, shellEnv, commitMsg, log); err != nil {
		return err
	}
	return pushReleaseBranchToOrigin(ctx, workDir, shellEnv, branch, allowRepeat, log)
}
