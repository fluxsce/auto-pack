package project

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"auto-pack/backend/internal/httpx"
	"auto-pack/backend/internal/repository"
	"auto-pack/backend/internal/web/middleware"
	"auto-pack/backend/pkg/audit"
	"auto-pack/backend/pkg/git"
	"auto-pack/backend/pkg/projectfs"
	"auto-pack/backend/pkg/projectrelease"

	"github.com/google/uuid"
)

type batchBranchClonePayload struct {
	WorkspaceRecID string `json:"workspace_rec_id"`
	SourceBranch   string `json:"source_branch"`
	TargetBranch   string `json:"target_branch"`
}

type batchBranchCloneStartResponse struct {
	Status         string `json:"status"`
	BatchRecID     string `json:"batch_rec_id"`
	WorkspaceRecID string `json:"workspace_rec_id"`
	WorkspaceName  string `json:"workspace_name,omitempty"`
	SourceBranch   string `json:"source_branch,omitempty"`
	TargetBranch   string `json:"target_branch,omitempty"`
}

func (h *Handler) batchBranchClone(w http.ResponseWriter, r *http.Request) {
	var body batchBranchClonePayload
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	wsID := strings.TrimSpace(body.WorkspaceRecID)
	if wsID == "" {
		httpx.WriteError(w, http.StatusBadRequest, "workspace_rec_id is required")
		return
	}
	source := git.NormalizeRepoBranchForStorage(body.SourceBranch)
	target := git.NormalizeRepoBranchForStorage(body.TargetBranch)
	if source == "" || target == "" {
		httpx.WriteError(w, http.StatusBadRequest, "source_branch and target_branch are required")
		return
	}
	if strings.EqualFold(source, target) {
		httpx.WriteError(w, http.StatusBadRequest, "source and target branch must differ")
		return
	}
	if git.IsTagRef(target) {
		httpx.WriteError(w, http.StatusBadRequest, "target branch cannot be a tag ref")
		return
	}
	body.SourceBranch = source
	body.TargetBranch = target

	ws, err := h.workspaces.FindByRecID(wsID)
	if err != nil {
		writeWorkspaceLoadError(w, err)
		return
	}
	projects, err := h.loadGitCloneProjects(wsID)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if len(projects) == 0 {
		httpx.WriteError(w, http.StatusBadRequest, "no Git projects in workspace")
		return
	}

	batchRecID, logPath, err := h.beginWorkspaceBranchCloneRecord(ws, body)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	auditUserRecID, _ := middleware.RecIDFromContext(r.Context())

	meta := projectrelease.BatchBranchCloneJob{
		BatchRecID:   batchRecID,
		SourceBranch: source,
		TargetBranch: target,
	}
	if err := h.branchCloneCoord.Start(ws.RecID, ws.WorkspaceName, len(projects), meta, func(ctx context.Context, job *projectrelease.BatchBranchCloneJob) {
		h.runBatchBranchClone(ctx, r, auditUserRecID, ws, projects, body, job, logPath)
	}); err != nil {
		fin := time.Now().UTC()
		_ = h.branchClones.Finish(batchRecID, repository.BranchCloneStatusFailed, err.Error(), logPath, 0, 0, 0, &fin)
		if errors.Is(err, projectrelease.ErrBranchCloneInProgress) {
			active := h.branchCloneCoord.Status(wsID)
			msg := "分支克隆正在进行中"
			if active.ActiveWorkspace != "" {
				msg += "（工作空间：" + active.ActiveWorkspace + "）"
			}
			httpx.WriteError(w, http.StatusConflict, msg)
			return
		}
		httpx.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	httpx.WriteJSON(w, http.StatusAccepted, batchBranchCloneStartResponse{
		Status:         projectrelease.JobStatusRunning,
		BatchRecID:     batchRecID,
		WorkspaceRecID: ws.RecID,
		WorkspaceName:  ws.WorkspaceName,
		SourceBranch:   source,
		TargetBranch:   target,
	})
}

func (h *Handler) beginWorkspaceBranchCloneRecord(
	ws *repository.PackWorkspace,
	body batchBranchClonePayload,
) (recID, logPath string, err error) {
	num, err := h.branchClones.NextBatchNum(ws.RecID)
	if err != nil {
		return "", "", err
	}
	recID = uuid.New().String()
	logDir := filepath.Join(h.dataDir, "branch-clones")
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		return "", "", err
	}
	logPath = filepath.Join(logDir, recID+".log")
	now := time.Now().UTC()
	item := &repository.PackWorkspaceBranchClone{
		RecID:          recID,
		WorkspaceRecID: ws.RecID,
		BatchNum:       num,
		Status:         repository.BranchCloneStatusRunning,
		SourceBranch:   body.SourceBranch,
		TargetBranch:   body.TargetBranch,
		Message:        "running",
		LogPath:        logPath,
		StartedAt:      &now,
	}
	if err := h.branchClones.Insert(item); err != nil {
		return "", "", err
	}
	return recID, logPath, nil
}

func (h *Handler) runBatchBranchClone(
	ctx context.Context,
	r *http.Request,
	auditUserRecID string,
	ws *repository.PackWorkspace,
	projects []repository.PackProject,
	body batchBranchClonePayload,
	job *projectrelease.BatchBranchCloneJob,
	logPath string,
) {
	cloner := &projectrelease.BranchCloner{
		Git: &projectrelease.GitCommitter{EnvRepo: h.env},
	}

	var results []projectrelease.BranchCloneResult
	cloned, skipped, failed := 0, 0, 0
	for i := range projects {
		p := &projects[i]
		h.branchCloneCoord.SetProgress(p.ProjectName, len(results))
		h.persistBranchCloneLogSnapshot(logPath, body, job.BatchRecID, results, cloned, skipped, failed, p.ProjectName)

		var res projectrelease.BranchCloneResult
		if p.IsEnabled != 1 {
			res = projectrelease.BranchCloneResult{
				ProjectName: p.ProjectName,
				Status:      "skipped",
				Message:     "project disabled",
			}
			skipped++
		} else {
			workDir := projectfs.ResolveProjectAbsDir(p, ws, h.dataDir)
			release := h.runner.LockProjectBuild(p.RecID)
			res = cloner.Run(ctx, p, workDir, body.SourceBranch, body.TargetBranch)
			release()
			switch res.Status {
			case "ok":
				cloned++
				p.RepoBranch = body.TargetBranch
				if err := h.projects.Update(p); err != nil {
					res.Message = res.Message + "; 工程分支同步失败: " + err.Error()
				}
			case "skipped":
				skipped++
			default:
				failed++
			}
		}
		results = append(results, res)
		h.branchCloneCoord.AppendResult(res, cloned, skipped, failed)
		_ = h.branchClones.UpdateProgress(job.BatchRecID, cloned, skipped, failed)
	}
	h.branchCloneCoord.SetProgress("", len(projects))
	h.branchCloneCoord.FinishResult(cloned, skipped, failed, results)

	fin := time.Now().UTC()
	status := repository.ResolveBranchCloneStatus(cloned, skipped, failed)
	summary := "cloned=" + strconv.Itoa(cloned) + " skipped=" + strconv.Itoa(skipped) + " failed=" + strconv.Itoa(failed)
	logText := h.formatPersistedBranchCloneLog(body, results, cloned, skipped, failed, true, "")
	_ = os.WriteFile(logPath, []byte(logText), 0o644)
	_ = h.branchClones.Finish(job.BatchRecID, status, summary, logPath, cloned, skipped, failed, &fin)

	h.audit.Record(r.Context(), r, audit.Entry{
		UserRecID: auditUserRecID,
		Action:    audit.ActionUpdate, Module: audit.ModuleProject, TargetType: "workspace",
		TargetID: ws.RecID, TargetName: ws.WorkspaceName,
		Summary: "批量分支克隆 " + body.SourceBranch + " → " + body.TargetBranch,
		Detail:  summary,
		Success: failed == 0,
	})
}

func (h *Handler) persistBranchCloneLogSnapshot(
	logPath string,
	body batchBranchClonePayload,
	batchRecID string,
	results []projectrelease.BranchCloneResult,
	cloned, skipped, failed int,
	currentProject string,
) {
	if batchRecID == "" || logPath == "" {
		return
	}
	logText := h.formatPersistedBranchCloneLog(body, results, cloned, skipped, failed, false, currentProject)
	_ = os.WriteFile(logPath, []byte(logText), 0o644)
}

func (h *Handler) formatPersistedBranchCloneLog(
	body batchBranchClonePayload,
	results []projectrelease.BranchCloneResult,
	cloned, skipped, failed int,
	final bool,
	currentProject string,
) string {
	return projectrelease.FormatBatchReleaseLog(projectrelease.BatchLogParams{
		StartLabel:        "开始批量分支克隆…",
		DoneLabel:         "分支克隆结束",
		ProgressLabel:     "克隆进度（实时）",
		ParamsLine:        "源分支: " + body.SourceBranch + " → 目标分支: " + body.TargetBranch,
		SummaryLine:       "汇总 — 成功 " + strconv.Itoa(cloned) + "，跳过 " + strconv.Itoa(skipped) + "，失败 " + strconv.Itoa(failed),
		NoStepsLabel:      "（无步骤明细）",
		NoMessageLabel:    "（无错误详情）",
		InProgressLabel:   "正在处理：%s",
		InProgressProject: currentProject,
		Final:             final,
		Running:           !final,
		Released:          cloned,
		Skipped:           skipped,
		Failed:            failed,
		Results:           projectrelease.BranchCloneResultsAsReleaseResults(results),
	})
}

func (h *Handler) loadGitCloneProjects(wsID string) ([]repository.PackProject, error) {
	all, err := h.projects.List(wsID, "")
	if err != nil {
		return nil, err
	}
	ws, err := h.workspaces.FindByRecID(wsID)
	if err != nil {
		return nil, err
	}
	out := make([]repository.PackProject, 0, len(all))
	for _, p := range all {
		if p.IsEnabled != 1 {
			continue
		}
		if strings.TrimSpace(p.RepoURL) == "" {
			continue
		}
		if projectfs.ResolveProjectAbsDir(&p, ws, h.dataDir) == "" {
			continue
		}
		out = append(out, p)
	}
	return out, nil
}
