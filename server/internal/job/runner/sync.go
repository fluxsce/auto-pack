package runner

import (
	"context"
	"fmt"
	"io"
	"strings"

	"auto-pack/server/internal/host/git"
	"auto-pack/server/internal/job"
	"auto-pack/server/internal/project"
)

func (e *Engine) stepSync(ctx context.Context, j job.Job, prj project.Project, p job.Payload, work string, w io.Writer) error {
	if skip, reason := skipSync(prj, p); skip {
		_, _ = fmt.Fprintln(w, reason)
		return nil
	}
	url := p.CloneURL
	branch := p.Branch
	if url == "" {
		url = prj.RepoURL
	}
	if branch == "" {
		branch = prj.RepoBranch
	}
	recipe := project.ParseRecipe(prj.RecipeJSON)
	opt := e.gitOptFor(ctx, j.WorkspaceRecID, recipe.GitEnvRecID)
	if git.IsRepo(work) {
		return git.FetchPull(ctx, work, branch, opt, w)
	}
	return git.Clone(ctx, url, work, branch, opt, w)
}

func (e *Engine) stepBranchClone(ctx context.Context, j job.Job, prj project.Project, p job.Payload, work string, w io.Writer) error {
	recipe := project.ParseRecipe(prj.RecipeJSON)
	if recipe.IsLocal() {
		_, _ = fmt.Fprintln(w, "scm_mode=local; skip branch clone")
		return nil
	}
	to := strings.TrimSpace(p.Branch)
	if to == "" {
		return fmt.Errorf("branch is required")
	}
	from := strings.TrimSpace(p.FromBranch)
	if from == "" {
		from = strings.TrimSpace(prj.RepoBranch)
	}
	if from == "" {
		return fmt.Errorf("source branch is required")
	}
	url := strings.TrimSpace(p.CloneURL)
	if url == "" {
		url = strings.TrimSpace(prj.RepoURL)
	}
	if url == "" {
		return fmt.Errorf("repo_url required")
	}
	opt := e.gitOptFor(ctx, j.WorkspaceRecID, recipe.GitEnvRecID)
	logLine(w, "克隆分支 %s -> %s（工程配置分支 %s）", from, to, dash(prj.RepoBranch))
	if cfg := strings.TrimSpace(prj.RepoBranch); cfg != "" && cfg != from {
		logLine(w, "源分支与工程配置分支不同，以源分支 %s 的远程最新为准", from)
	}
	if !git.IsRepo(work) {
		if err := git.Clone(ctx, url, work, from, opt, w); err != nil {
			return err
		}
	}
	if p.ForcePush() {
		logLine(w, "目标分支已存在时覆盖远程 origin/%s", to)
	} else {
		logLine(w, "目标分支已存在且不能快进时推送将失败")
	}
	if err := git.CreateBranch(ctx, work, from, to, p.ForcePush(), opt, w); err != nil {
		return err
	}
	if e.projects == nil || prj.RecID == "" {
		return nil
	}
	return e.projects.SetRepoBranch(ctx, prj.RecID, to)
}

// skipSync reports that the sync step should no-op (local checkout, disabled pull, or no URL).
func skipSync(prj project.Project, p job.Payload) (bool, string) {
	recipe := project.ParseRecipe(prj.RecipeJSON)
	if recipe.IsLocal() {
		return true, "scm_mode=local; skip sync"
	}
	if !recipe.ShouldSync() {
		return true, "auto_sync=false; skip sync"
	}
	url := strings.TrimSpace(p.CloneURL)
	if url == "" {
		url = strings.TrimSpace(prj.RepoURL)
	}
	if url == "" {
		return true, "no repo_url; skip sync"
	}
	return false, ""
}
