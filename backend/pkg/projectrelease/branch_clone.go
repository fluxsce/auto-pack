package projectrelease

import (
	"context"
	"fmt"
	"strings"

	"auto-pack/backend/internal/repository"
	apigit "auto-pack/backend/pkg/git"
)

// BranchCloner copies a source git ref to a target branch on origin for one project.
type BranchCloner struct {
	Git *GitCommitter
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
	if err := cloneBranchFromSource(ctx, workDir, shellEnv, sourceBranch, targetBranch, log); err != nil {
		return BranchCloneResult{ProjectName: name, Status: "failed", Message: err.Error(), Steps: steps}
	}
	log.step("分支克隆完成（%s → %s）", sourceBranch, targetBranch)
	return BranchCloneResult{ProjectName: name, Status: "ok", Steps: steps}
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
		return pushBranchFromHEAD(ctx, dir, env, targetBranch, log)
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
	return pushBranchFromHEAD(ctx, dir, env, targetBranch, log)
}
