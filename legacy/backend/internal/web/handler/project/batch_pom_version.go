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
	"auto-pack/backend/pkg/projectfs"
	"auto-pack/backend/pkg/projectrelease"

	"github.com/google/uuid"
)

type batchPomVersionPayload struct {
	WorkspaceRecID string `json:"workspace_rec_id"`
	VersionTag     string `json:"version_tag"`
	CommitMessage  string `json:"commit_message"`
	AllowRepeat    *bool  `json:"allow_repeat"`
}

type batchPomVersionStartResponse struct {
	Status         string `json:"status"`
	BatchRecID     string `json:"batch_rec_id"`
	WorkspaceRecID string `json:"workspace_rec_id"`
	WorkspaceName  string `json:"workspace_name,omitempty"`
	Tag            string `json:"tag,omitempty"`
}

func (h *Handler) batchPomVersion(w http.ResponseWriter, r *http.Request) {
	var body batchPomVersionPayload
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	wsID := strings.TrimSpace(body.WorkspaceRecID)
	if wsID == "" {
		httpx.WriteError(w, http.StatusBadRequest, "workspace_rec_id is required")
		return
	}
	tag := projectrelease.NormalizeVersionTag(body.VersionTag)
	if tag == "" {
		httpx.WriteError(w, http.StatusBadRequest, "version_tag is required")
		return
	}
	if err := projectrelease.ValidateVersionTag(tag); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	body.VersionTag = tag
	body.CommitMessage = strings.TrimSpace(body.CommitMessage)

	ws, err := h.workspaces.FindByRecID(wsID)
	if err != nil {
		writeWorkspaceLoadError(w, err)
		return
	}
	projects, err := h.loadMavenProjects(wsID)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if len(projects) == 0 {
		httpx.WriteError(w, http.StatusBadRequest, "no Maven projects in workspace")
		return
	}

	batchRecID, logPath, err := h.beginWorkspacePomVersionRecord(ws, body)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	auditUserRecID, _ := middleware.RecIDFromContext(r.Context())

	meta := projectrelease.BatchPomVersionJob{
		BatchRecID:    batchRecID,
		Tag:           tag,
		CommitMessage: body.CommitMessage,
	}
	if err := h.pomVersionCoord.Start(ws.RecID, ws.WorkspaceName, len(projects), meta, func(ctx context.Context, job *projectrelease.BatchPomVersionJob) {
		h.runBatchPomVersion(ctx, r, auditUserRecID, ws, projects, body, job, logPath)
	}); err != nil {
		fin := time.Now().UTC()
		_ = h.pomVersions.Finish(batchRecID, repository.PomVersionStatusFailed, err.Error(), logPath, 0, 0, 0, &fin)
		if errors.Is(err, projectrelease.ErrPomVersionInProgress) {
			active := h.pomVersionCoord.Status(wsID)
			msg := "批量 POM 版本更新正在进行中"
			if active.ActiveWorkspace != "" {
				msg += "（工作空间：" + active.ActiveWorkspace + "）"
			}
			httpx.WriteError(w, http.StatusConflict, msg)
			return
		}
		httpx.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	httpx.WriteJSON(w, http.StatusAccepted, batchPomVersionStartResponse{
		Status:         projectrelease.JobStatusRunning,
		BatchRecID:     batchRecID,
		WorkspaceRecID: ws.RecID,
		WorkspaceName:  ws.WorkspaceName,
		Tag:            tag,
	})
}

func (h *Handler) beginWorkspacePomVersionRecord(
	ws *repository.PackWorkspace,
	body batchPomVersionPayload,
) (recID, logPath string, err error) {
	num, err := h.pomVersions.NextBatchNum(ws.RecID)
	if err != nil {
		return "", "", err
	}
	recID = uuid.New().String()
	logDir := filepath.Join(h.dataDir, "pom-versions")
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		return "", "", err
	}
	logPath = filepath.Join(logDir, recID+".log")
	now := time.Now().UTC()
	item := &repository.PackWorkspacePomVersion{
		RecID:          recID,
		WorkspaceRecID: ws.RecID,
		BatchNum:       num,
		Status:         repository.PomVersionStatusRunning,
		VersionTag:     body.VersionTag,
		CommitMessage:  body.CommitMessage,
		Message:        "running",
		LogPath:        logPath,
		StartedAt:      &now,
	}
	if err := h.pomVersions.Insert(item); err != nil {
		return "", "", err
	}
	return recID, logPath, nil
}

func (h *Handler) runBatchPomVersion(
	ctx context.Context,
	r *http.Request,
	auditUserRecID string,
	ws *repository.PackWorkspace,
	projects []repository.PackProject,
	body batchPomVersionPayload,
	job *projectrelease.BatchPomVersionJob,
	logPath string,
) {
	allowRepeat := true
	if body.AllowRepeat != nil {
		allowRepeat = *body.AllowRepeat
	}

	updater := &projectrelease.PomVersionUpdater{
		Git: &projectrelease.GitCommitter{EnvRepo: h.env},
	}

	var results []projectrelease.PomVersionResult
	updated, skipped, failed := 0, 0, 0
	for i := range projects {
		p := &projects[i]
		h.pomVersionCoord.SetProgress(p.ProjectName, len(results))
		h.persistPomVersionLogSnapshot(logPath, body, job.BatchRecID, results, updated, skipped, failed, p.ProjectName)

		var res projectrelease.PomVersionResult
		if !projectrelease.IsMavenReleaseProject(p.ProjectType) {
			continue
		}
		if p.IsEnabled != 1 {
			res = projectrelease.PomVersionResult{
				ProjectName: p.ProjectName,
				Status:      "skipped",
				Message:     "project disabled",
			}
			skipped++
		} else {
			workDir := projectfs.ResolveProjectSourceAbsDir(p, ws, h.dataDir)
			res = updater.Run(ctx, p, projectrelease.PomVersionRequest{
				WorkDir:       workDir,
				Workspace:     ws,
				DataDir:       h.dataDir,
				Tag:           body.VersionTag,
				CommitMessage: body.CommitMessage,
				AllowRepeat:   allowRepeat,
			})
			switch res.Status {
			case "ok":
				updated++
				p.ProjectVersion = body.VersionTag
				if err := h.projects.Update(p); err != nil {
					res.Message = res.Message + "; 工程版本同步失败: " + err.Error()
				}
			case "skipped":
				skipped++
			default:
				failed++
			}
		}
		results = append(results, res)
		h.pomVersionCoord.AppendResult(res, updated, skipped, failed)
		_ = h.pomVersions.UpdateProgress(job.BatchRecID, updated, skipped, failed)
	}
	h.pomVersionCoord.SetProgress("", len(projects))
	h.pomVersionCoord.FinishResult(updated, skipped, failed, results)

	fin := time.Now().UTC()
	status := repository.ResolvePomVersionStatus(updated, skipped, failed)
	summary := "updated=" + strconv.Itoa(updated) + " skipped=" + strconv.Itoa(skipped) + " failed=" + strconv.Itoa(failed)
	logText := h.formatPersistedPomVersionLog(body, results, updated, skipped, failed, true, "")
	_ = os.WriteFile(logPath, []byte(logText), 0o644)
	_ = h.pomVersions.Finish(job.BatchRecID, status, summary, logPath, updated, skipped, failed, &fin)

	h.audit.Record(r.Context(), r, audit.Entry{
		UserRecID: auditUserRecID,
		Action:    audit.ActionUpdate, Module: audit.ModuleProject, TargetType: "workspace",
		TargetID: ws.RecID, TargetName: ws.WorkspaceName,
		Summary: "批量更新 POM 版本 " + body.VersionTag,
		Detail:  summary,
		Success: failed == 0,
	})
}

func (h *Handler) persistPomVersionLogSnapshot(
	logPath string,
	body batchPomVersionPayload,
	batchRecID string,
	results []projectrelease.PomVersionResult,
	updated, skipped, failed int,
	currentProject string,
) {
	if batchRecID == "" || logPath == "" {
		return
	}
	logText := h.formatPersistedPomVersionLog(body, results, updated, skipped, failed, false, currentProject)
	_ = os.WriteFile(logPath, []byte(logText), 0o644)
}

func (h *Handler) formatPersistedPomVersionLog(
	body batchPomVersionPayload,
	results []projectrelease.PomVersionResult,
	updated, skipped, failed int,
	final bool,
	currentProject string,
) string {
	commit := body.CommitMessage
	if commit == "" {
		commit = "（自动生成）"
	}
	return projectrelease.FormatBatchReleaseLog(projectrelease.BatchLogParams{
		StartLabel:        "开始批量更新 POM 版本…",
		DoneLabel:         "批量更新结束",
		ProgressLabel:     "更新进度（实时）",
		ParamsLine:        "版本 Tag: " + body.VersionTag,
		NoteLine:          "提交说明: " + commit,
		SummaryLine:       "汇总 — 成功 " + strconv.Itoa(updated) + "，跳过 " + strconv.Itoa(skipped) + "，失败 " + strconv.Itoa(failed) + "（" + body.VersionTag + "）",
		NoStepsLabel:      "（无步骤明细）",
		NoMessageLabel:    "（无错误详情）",
		InProgressLabel:   "正在处理：%s",
		InProgressProject: currentProject,
		Final:             final,
		Running:           !final,
		Released:          updated,
		Skipped:           skipped,
		Failed:            failed,
		Results:           projectrelease.PomVersionResultsAsReleaseResults(results),
	})
}

func (h *Handler) loadMavenProjects(wsID string) ([]repository.PackProject, error) {
	all, err := h.projects.List(wsID, "maven")
	if err != nil {
		return nil, err
	}
	out := make([]repository.PackProject, 0, len(all))
	for _, p := range all {
		if projectrelease.IsMavenReleaseProject(p.ProjectType) {
			out = append(out, p)
		}
	}
	return out, nil
}
