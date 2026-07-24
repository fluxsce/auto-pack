package packrun

import (
	"context"
	"fmt"
	"os"
	"strings"

	"auto-pack/backend/internal/repository"
	"auto-pack/backend/pkg/projectcfg"
	"auto-pack/backend/pkg/projectfs"
)

// WorkspaceSyncResult summarizes a workspace-wide git sync (clone/fetch + checkout).
type WorkspaceSyncResult struct {
	Synced  int      `json:"synced"`
	Skipped int      `json:"skipped"`
	Failed  []string `json:"failed,omitempty"`
}

// StartWorkspaceSourceSync begins async git clone/fetch for all enabled Git projects in the workspace.
func (r *Runner) StartWorkspaceSourceSync(workspaceRecID, workspaceName string) error {
	return r.syncCoord.start(workspaceRecID, workspaceName, r.runWorkspaceSourceSync)
}

// WorkspaceSourceSyncStatus returns progress for polling (queryWorkspace is the caller's workspace).
func (r *Runner) WorkspaceSourceSyncStatus(queryWorkspace string) WorkspaceSourceSyncJob {
	return r.syncCoord.status(queryWorkspace)
}

// WorkspaceSourceSyncCapacity returns running sync count, max allowed, and active workspace names.
func (r *Runner) WorkspaceSourceSyncCapacity() (running, max int, names []string) {
	return r.syncCoord.RunningSyncSummary()
}

// SyncWorkspaceSources clones or updates every enabled project with a Git remote in the workspace.
func (r *Runner) SyncWorkspaceSources(ctx context.Context, workspaceRecID string) (*WorkspaceSyncResult, error) {
	ws, projects, err := r.listWorkspaceSyncProjects(workspaceRecID)
	if err != nil {
		return nil, err
	}
	return r.syncWorkspaceProjects(ctx, ws, projects, nil)
}

func (r *Runner) runWorkspaceSourceSync(ctx context.Context, job *WorkspaceSourceSyncJob) {
	wsID := job.WorkspaceRecID
	ws, projects, err := r.listWorkspaceSyncProjects(wsID)
	if err != nil {
		r.syncCoord.recordOutcome(wsID, false, false, err.Error())
		return
	}
	all, err := r.projects.List(wsID, "")
	if err != nil {
		r.syncCoord.recordOutcome(wsID, false, false, err.Error())
		return
	}
	r.syncCoord.setInitialSkipped(wsID, countWorkspaceSyncSkipped(all, ws, r.dataDir))
	total := len(projects)
	r.syncCoord.setProgress(wsID, "", 0, total)
	progress := func(current string, done int) {
		r.syncCoord.setProgress(wsID, current, done, total)
	}
	_, _ = r.syncWorkspaceProjects(ctx, ws, projects, progress)
}

func countWorkspaceSyncSkipped(all []repository.PackProject, ws *repository.PackWorkspace, dataDir string) int {
	skipped := 0
	for i := range all {
		p := &all[i]
		if p.IsEnabled != 1 {
			skipped++
			continue
		}
		if strings.TrimSpace(p.RepoURL) == "" {
			skipped++
			continue
		}
		if projectfs.ResolveProjectAbsDir(p, ws, dataDir) == "" {
			skipped++
		}
	}
	return skipped
}

func (r *Runner) listWorkspaceSyncProjects(workspaceRecID string) (*repository.PackWorkspace, []repository.PackProject, error) {
	workspaceRecID = strings.TrimSpace(workspaceRecID)
	if workspaceRecID == "" {
		return nil, nil, fmt.Errorf("workspace_rec_id is required")
	}
	ws, err := r.workspaces.FindByRecID(workspaceRecID)
	if err != nil {
		return nil, nil, err
	}
	all, err := r.projects.List(workspaceRecID, "")
	if err != nil {
		return nil, nil, err
	}
	var eligible []repository.PackProject
	for i := range all {
		p := all[i]
		if p.IsEnabled != 1 {
			continue
		}
		if !projectcfg.UsesGitSource(&p) {
			continue
		}
		if projectfs.ResolveProjectSourceAbsDir(&p, ws, r.dataDir) == "" {
			continue
		}
		eligible = append(eligible, p)
	}
	return ws, eligible, nil
}

func (r *Runner) syncWorkspaceProjects(
	ctx context.Context,
	ws *repository.PackWorkspace,
	projects []repository.PackProject,
	onProgress func(current string, done int),
) (*WorkspaceSyncResult, error) {
	log, closeLog := discardBuildLogger()
	defer closeLog()
	res := &WorkspaceSyncResult{}
	wsID := ""
	if ws != nil {
		wsID = ws.RecID
	}
	for i := range projects {
		p := &projects[i]
		if onProgress != nil {
			onProgress(p.ProjectName, i)
		}
		workDir := projectfs.ResolveProjectSourceAbsDir(p, ws, r.dataDir)
		release := r.acquireProjectBuild(p.RecID)
		err := r.syncSource(ctx, p, workDir, log, &syncSourceOpts{compact: true})
		release()
		if onProgress != nil {
			onProgress(p.ProjectName, i+1)
		}
		if err != nil {
			msg := fmt.Sprintf("%s: %s", p.ProjectName, err.Error())
			res.Failed = append(res.Failed, msg)
			r.syncCoord.recordOutcome(wsID, false, false, msg)
			continue
		}
		res.Synced++
		r.syncCoord.recordOutcome(wsID, true, false, "")
	}
	return res, nil
}

func discardBuildLogger() (*buildLogger, func()) {
	f, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		return newBuildLogger(os.Stdout), func() {}
	}
	return newBuildLogger(f), func() { _ = f.Close() }
}
