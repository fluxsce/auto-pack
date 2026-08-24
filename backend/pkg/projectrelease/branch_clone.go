package projectrelease

import (
	"context"
	"fmt"
	"strings"

	"auto-pack/backend/internal/repository"
	apigit "auto-pack/backend/pkg/git"
)

// ExistingTargetPolicy controls what happens when the target branch already exists on origin.
type ExistingTargetPolicy string

const (
	// ExistingTargetOverwrite deletes the remote target branch, then recreates it from the source tip.
	ExistingTargetOverwrite ExistingTargetPolicy = "overwrite"
	// ExistingTargetSkip leaves an existing remote target branch unchanged.
	ExistingTargetSkip ExistingTargetPolicy = "skip"
)

func NormalizeExistingTargetPolicy(raw string) ExistingTargetPolicy {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case string(ExistingTargetSkip), "skip_existing", "skip-existing":
		return ExistingTargetSkip
	default:
		return ExistingTargetOverwrite
	}
}

// protectedBranchesForDelete must never be deleted by branch-clone overwrite.
var protectedBranchesForDelete = map[string]struct{}{
	"develop": {},
	"master":  {},
	"main":    {},
}

// IsProtectedBranchForDelete reports whether branch is develop/master/main (case-insensitive).
func IsProtectedBranchForDelete(branch string) bool {
	name := strings.ToLower(strings.TrimSpace(branch))
	if name == "" {
		return false
	}
	_, ok := protectedBranchesForDelete[name]
	return ok
}

// BranchCloner copies a source git ref to a target branch on origin for one project.
type BranchCloner struct {
	Git *GitCommitter
	// ExistingTarget selects overwrite (default) or skip when target already exists on origin.
	ExistingTarget ExistingTargetPolicy
}

// Run clones sourceBranch to targetBranch for the project worktree and pushes to origin.
func (c *BranchCloner) Run(
	ctx context.Context,
	project *repository.PackProject,
	workDir, sourceBranch, targetBranch string,
) BranchCloneResult {
	name := strings.TrimSpace(project.ProjectName)
	var steps []string
	log := newPublishLogger(&steps)
	sourceBranch = strings.TrimSpace(sourceBranch)
	targetBranch = strings.TrimSpace(targetBranch)
	policy := NormalizeExistingTargetPolicy(string(c.ExistingTarget))
	if sourceBranch == "" || targetBranch == "" {
		return BranchCloneResult{
			ProjectName: name,
			Status:      "failed",
			Message:     "source and target branch are required",
			Steps:       steps,
		}
	}
	if strings.EqualFold(sourceBranch, targetBranch) {
		return BranchCloneResult{
			ProjectName: name,
			Status:      "failed",
			Message:     "source and target branch must differ",
			Steps:       steps,
		}
	}
	if apigit.IsTagRef(targetBranch) {
		return BranchCloneResult{
			ProjectName: name,
			Status:      "failed",
			Message:     "target cannot be a tag ref",
			Steps:       steps,
		}
	}
	if IsProtectedBranchForDelete(targetBranch) && policy == ExistingTargetOverwrite {
		return BranchCloneResult{
			ProjectName: name,
			Status:      "failed",
			Message:     fmt.Sprintf("refusing to delete protected branch %q (develop/master/main are not allowed as overwrite targets)", targetBranch),
			Steps:       steps,
		}
	}
	if strings.TrimSpace(project.RepoURL) == "" {
		return BranchCloneResult{
			ProjectName: name,
			Status:      "skipped",
			Message:     "no repository URL configured",
			Steps:       steps,
		}
	}
	if c.Git == nil {
		c.Git = &GitCommitter{}
	}
	log.step("分支克隆开始（%s → %s）", sourceBranch, targetBranch)
	shellEnv, err := c.Git.shellEnv(project)
	if err != nil {
		return BranchCloneResult{ProjectName: name, Status: "failed", Message: err.Error(), Steps: steps}
	}
	cloneBranch := sourceBranch
	if apigit.IsTagRef(cloneBranch) {
		cloneBranch = apigit.CloneBranchForSync(cloneBranch)
	}
	if err := c.Git.EnsureGitRepository(ctx, project, workDir, cloneBranch, log); err != nil {
		return BranchCloneResult{ProjectName: name, Status: "failed", Message: err.Error(), Steps: steps}
	}
	if _, err := apigit.Open(workDir); err != nil {
		return BranchCloneResult{ProjectName: name, Status: "failed", Message: err.Error(), Steps: steps}
	}
	if err := syncOriginRefs(ctx, workDir, shellEnv, log); err != nil {
		return BranchCloneResult{ProjectName: name, Status: "failed", Message: err.Error(), Steps: steps}
	}

	exists, err := originHeadBranchExists(ctx, workDir, shellEnv, targetBranch)
	if err != nil {
		return BranchCloneResult{ProjectName: name, Status: "failed", Message: err.Error(), Steps: steps}
	}
	if exists {
		if policy == ExistingTargetSkip {
			log.step("目标分支 %s 已存在于远程，按策略跳过", targetBranch)
			return BranchCloneResult{
				ProjectName: name,
				Status:      "skipped",
				Message:     "target branch already exists on origin",
				Steps:       steps,
			}
		}
		log.step("目标分支 %s 已存在于远程，先删除再从源分支重建", targetBranch)
		if err := deleteOriginBranch(ctx, workDir, shellEnv, sourceBranch, targetBranch, log); err != nil {
			return BranchCloneResult{ProjectName: name, Status: "failed", Message: err.Error(), Steps: steps}
		}
	}

	if err := cloneBranchFromSource(ctx, workDir, shellEnv, sourceBranch, targetBranch, log); err != nil {
		return BranchCloneResult{ProjectName: name, Status: "failed", Message: err.Error(), Steps: steps}
	}
	log.step("分支克隆完成（%s → %s）", sourceBranch, targetBranch)
	return BranchCloneResult{ProjectName: name, Status: "ok", Steps: steps}
}

// deleteOriginBranch removes the remote target branch (and local tracking refs) so a later
// push from source creates a clean tip equal to the source ref.
func deleteOriginBranch(
	ctx context.Context,
	dir string,
	env []string,
	sourceBranch, targetBranch string,
	log *publishLogger,
) error {
	targetBranch = strings.TrimSpace(targetBranch)
	sourceBranch = strings.TrimSpace(sourceBranch)
	if targetBranch == "" {
		return fmt.Errorf("target branch is empty")
	}
	if IsProtectedBranchForDelete(targetBranch) {
		return fmt.Errorf("refusing to delete protected branch %q", targetBranch)
	}
	if strings.EqualFold(targetBranch, sourceBranch) {
		return fmt.Errorf("refusing to delete source branch %q", sourceBranch)
	}

	// Cannot delete the currently checked-out branch; move to source tip first.
	if cur := currentGitBranch(ctx, dir, env); strings.EqualFold(cur, targetBranch) {
		safeRef := strings.TrimSpace(sourceBranch)
		if apigit.IsTagRef(safeRef) {
			safeRef = apigit.NormalizeTagName(safeRef)
			ensureGitTagFetched(ctx, dir, env, safeRef)
			for _, ref := range apigit.CheckoutRefCandidates(sourceBranch) {
				if gitRefExists(ctx, dir, env, ref) {
					safeRef = ref
					break
				}
			}
		} else {
			_ = fetchOriginBranch(ctx, dir, env, safeRef, log)
			if gitRefExists(ctx, dir, env, originBranchRef(safeRef)) {
				safeRef = originBranchRef(safeRef)
			}
		}
		log.step("当前在目标分支，先切换到 %s 再删除", safeRef)
		if _, err := runGit(ctx, dir, env, "checkout", "-f", "--detach", safeRef); err != nil {
			if _, err2 := runGit(ctx, dir, env, "checkout", "-f", safeRef); err2 != nil {
				return fmt.Errorf("checkout away from %s before delete: %w", targetBranch, err2)
			}
		}
	}

	log.step("git push origin --delete %s", targetBranch)
	if _, err := runGit(ctx, dir, env, "push", "origin", "--delete", targetBranch); err != nil {
		// Fallback delete refspec for older remotes/clients.
		log.step("git push origin :refs/heads/%s", targetBranch)
		if _, err2 := runGit(ctx, dir, env, "push", "origin", ":refs/heads/"+targetBranch); err2 != nil {
			return fmt.Errorf("delete remote branch %s: %w", targetBranch, err)
		}
	}

	if _, err := runGitAllow(ctx, dir, env, "branch", "-D", targetBranch); err == nil {
		log.step("已删除本地分支 %s", targetBranch)
	}
	remoteRef := originRemoteBranchRef(targetBranch)
	if remoteRef != "" {
		if _, err := runGitAllow(ctx, dir, env, "update-ref", "-d", remoteRef); err == nil {
			log.step("已清理远程跟踪引用 %s", remoteRef)
		}
	}
	return nil
}

func cloneBranchFromSource(
	ctx context.Context,
	dir string,
	env []string,
	sourceBranch, targetBranch string,
	log *publishLogger,
) error {
	sourceBranch = strings.TrimSpace(sourceBranch)
	targetBranch = strings.TrimSpace(targetBranch)
	if apigit.IsTagRef(sourceBranch) {
		tagName := apigit.NormalizeTagName(sourceBranch)
		log.step("源为 Tag %s，拉取标签", tagName)
		ensureGitTagFetched(ctx, dir, env, tagName)
		var sourceRef string
		for _, ref := range apigit.CheckoutRefCandidates(sourceBranch) {
			if gitRefExists(ctx, dir, env, ref) {
				sourceRef = ref
				break
			}
		}
		if sourceRef == "" {
			return fmt.Errorf("source tag %s not found on origin", tagName)
		}
		log.step("git checkout -f -B %s %s", targetBranch, sourceRef)
		if err := checkoutNewBranchFromRef(ctx, dir, env, targetBranch, sourceRef); err != nil {
			return err
		}
		return pushNewBranchFromHEAD(ctx, dir, env, targetBranch, log)
	}

	onOrigin, err := originHeadBranchExists(ctx, dir, env, sourceBranch)
	if err != nil {
		return err
	}
	if !onOrigin {
		return fmt.Errorf("source branch %s does not exist on origin", sourceBranch)
	}
	if err := fetchOriginBranch(ctx, dir, env, sourceBranch, log); err != nil {
		return err
	}
	sourceRef := originBranchRef(sourceBranch)
	if !gitRefExists(ctx, dir, env, sourceRef) {
		return fmt.Errorf("origin/%s missing after fetch", sourceBranch)
	}
	log.step("从 %s 创建目标分支 %s", sourceRef, targetBranch)
	if err := checkoutNewBranchFromRef(ctx, dir, env, targetBranch, sourceRef); err != nil {
		return err
	}
	return pushNewBranchFromHEAD(ctx, dir, env, targetBranch, log)
}

// pushNewBranchFromHEAD publishes a freshly created local branch. After an overwrite delete,
// the remote should not exist; still allow force fallback for races.
func pushNewBranchFromHEAD(ctx context.Context, dir string, env []string, targetBranch string, log *publishLogger) error {
	targetBranch = strings.TrimSpace(targetBranch)
	if targetBranch == "" {
		return fmt.Errorf("target branch is empty")
	}
	if cur := currentGitBranch(ctx, dir, env); !strings.EqualFold(cur, targetBranch) {
		log.step("git branch -f %s HEAD", targetBranch)
		if _, err := runGit(ctx, dir, env, "branch", "-f", targetBranch, "HEAD"); err != nil {
			return err
		}
	}
	log.step("git push -u origin %s（新建目标分支）", targetBranch)
	if _, err := runGit(ctx, dir, env, "push", "-u", "origin", targetBranch); err != nil {
		// Rare race: someone recreated the branch between delete and push.
		log.step("普通推送失败，回退 force 推送源尖端")
		if err2 := pushBranchWithRepeatFallback(ctx, dir, env, targetBranch, true, log); err2 != nil {
			return err2
		}
	}
	log.step("已推送到目标分支 %s", targetBranch)
	return nil
}
