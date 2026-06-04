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

type batchAutoReleasePayload struct {
	WorkspaceRecID     string `json:"workspace_rec_id"`
	ReleaseBranch      string `json:"release_branch"`
	TagBranch          string `json:"tag_branch"`
	ReleaseNote        string `json:"release_note"`
	ReleaseDate        string `json:"release_date"`
	AllowRepeat        *bool  `json:"allow_repeat"`
	UpdatePomVersion   *bool  `json:"update_pom_version"`
	CreateTargetBranch *bool  `json:"create_target_branch"`
	TargetBranch       string `json:"target_branch"`
}

type batchAutoReleaseStartResponse struct {
	Status         string `json:"status"`
	ReleaseRecID   string `json:"release_rec_id"`
	WorkspaceRecID string `json:"workspace_rec_id"`
	WorkspaceName  string `json:"workspace_name,omitempty"`
	ReleaseBranch  string `json:"release_branch,omitempty"`
	Tag            string `json:"tag,omitempty"`
}

func (h *Handler) batchAutoRelease(w http.ResponseWriter, r *http.Request) {
	body, err := parseBatchAutoReleasePayload(r)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	ws, projects, err := h.loadWorkspaceProjects(body.WorkspaceRecID)
	if err != nil {
		writeWorkspaceLoadError(w, err)
		return
	}

	releaseRecID, logPath, err := h.beginWorkspaceReleaseRecord(ws, body)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	auditUserRecID, _ := middleware.RecIDFromContext(r.Context())

	meta := projectrelease.BatchReleaseJob{
		ReleaseRecID:  releaseRecID,
		Tag:           body.TagBranch,
		ReleaseBranch: body.ReleaseBranch,
	}
	if err := h.releaseCoord.Start(ws.RecID, ws.WorkspaceName, len(projects), meta, func(ctx context.Context, job *projectrelease.BatchReleaseJob) {
		h.runBatchAutoRelease(ctx, r, auditUserRecID, ws, projects, body, job, logPath)
	}); err != nil {
		fin := time.Now().UTC()
		_ = h.releases.Finish(releaseRecID, repository.ReleaseStatusFailed, err.Error(), logPath, 0, 0, 0, &fin)
		if errors.Is(err, projectrelease.ErrReleaseInProgress) {
			active := h.releaseCoord.Status(body.WorkspaceRecID)
			msg := "自动发版正在进行中"
			if active.ActiveWorkspace != "" {
				msg += "（工作空间：" + active.ActiveWorkspace + "）"
			}
			httpx.WriteError(w, http.StatusConflict, msg)
			return
		}
		httpx.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	httpx.WriteJSON(w, http.StatusAccepted, batchAutoReleaseStartResponse{
		Status:         projectrelease.JobStatusRunning,
		ReleaseRecID:   releaseRecID,
		WorkspaceRecID: ws.RecID,
		WorkspaceName:  ws.WorkspaceName,
		ReleaseBranch:  body.ReleaseBranch,
		Tag:            body.TagBranch,
	})
}

func (h *Handler) beginWorkspaceReleaseRecord(ws *repository.PackWorkspace, body batchAutoReleasePayload) (recID, logPath string, err error) {
	num, err := h.releases.NextReleaseNum(ws.RecID)
	if err != nil {
		return "", "", err
	}
	recID = uuid.New().String()
	logDir := filepath.Join(h.dataDir, "releases")
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		return "", "", err
	}
	logPath = filepath.Join(logDir, recID+".log")
	target := ""
	if body.CreateTargetBranch != nil && *body.CreateTargetBranch {
		target = strings.TrimSpace(body.TargetBranch)
	}
	now := time.Now().UTC()
	item := &repository.PackWorkspaceRelease{
		RecID:          recID,
		WorkspaceRecID: ws.RecID,
		ReleaseNum:     num,
		Status:         repository.ReleaseStatusRunning,
		ReleaseBranch:  body.ReleaseBranch,
		TagBranch:      body.TagBranch,
		TargetBranch:   target,
		Message:        "running",
		LogPath:        logPath,
		StartedAt:      &now,
	}
	if err := h.releases.Insert(item); err != nil {
		return "", "", err
	}
	return recID, logPath, nil
}

func (h *Handler) batchAutoReleaseStatus(w http.ResponseWriter, r *http.Request) {
	wsID := strings.TrimSpace(r.URL.Query().Get("workspace_rec_id"))
	if wsID == "" {
		httpx.WriteError(w, http.StatusBadRequest, "workspace_rec_id is required")
		return
	}
	st := h.releaseCoord.Status(wsID)
	httpx.WriteJSON(w, http.StatusOK, st)
}

func parseBatchAutoReleasePayload(r *http.Request) (batchAutoReleasePayload, error) {
	var body batchAutoReleasePayload
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		return body, errors.New("invalid request body")
	}
	body.WorkspaceRecID = strings.TrimSpace(body.WorkspaceRecID)
	if body.WorkspaceRecID == "" {
		return body, errors.New("workspace_rec_id is required")
	}
	body.ReleaseBranch = strings.TrimSpace(body.ReleaseBranch)
	if body.ReleaseBranch == "" {
		return body, errors.New("release_branch is required")
	}
	body.TagBranch = strings.TrimSpace(body.TagBranch)
	if body.TagBranch == "" {
		return body, errors.New("tag_branch is required")
	}
	body.ReleaseDate = strings.TrimSpace(body.ReleaseDate)
	if body.ReleaseDate == "" {
		body.ReleaseDate = time.Now().Format("2006-01-02")
	}
	body.TargetBranch = strings.TrimSpace(body.TargetBranch)
	createTarget := false
	if body.CreateTargetBranch != nil {
		createTarget = *body.CreateTargetBranch
	}
	if createTarget && body.TargetBranch == "" {
		return body, errors.New("target_branch is required when create_target_branch is enabled")
	}
	return body, nil
}

func (h *Handler) loadWorkspaceProjects(wsID string) (*repository.PackWorkspace, []repository.PackProject, error) {
	ws, err := h.workspaces.FindByRecID(wsID)
	if err != nil {
		return nil, nil, err
	}
	projects, err := h.projects.List(wsID, "")
	if err != nil {
		return nil, nil, err
	}
	if len(projects) == 0 {
		return nil, nil, errors.New("no projects in workspace")
	}
	return ws, projects, nil
}

func writeWorkspaceLoadError(w http.ResponseWriter, err error) {
	if errors.Is(err, repository.ErrWorkspaceNotFound) {
		httpx.WriteError(w, http.StatusNotFound, err.Error())
		return
	}
	if err.Error() == "no projects in workspace" {
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	httpx.WriteError(w, http.StatusInternalServerError, err.Error())
}

func (h *Handler) runBatchAutoRelease(
	ctx context.Context,
	r *http.Request,
	auditUserRecID string,
	ws *repository.PackWorkspace,
	projects []repository.PackProject,
	body batchAutoReleasePayload,
	job *projectrelease.BatchReleaseJob,
	logPath string,
) {
	allowRepeat := true
	if body.AllowRepeat != nil {
		allowRepeat = *body.AllowRepeat
	}
	updatePom := true
	if body.UpdatePomVersion != nil {
		updatePom = *body.UpdatePomVersion
	}
	createTarget := false
	if body.CreateTargetBranch != nil {
		createTarget = *body.CreateTargetBranch
	}

	releaser := &projectrelease.Releaser{
		Git: &projectrelease.GitCommitter{EnvRepo: h.env},
	}

	var results []projectrelease.ReleaseResult
	released, skipped, failed := 0, 0, 0
	for _, p := range projects {
		h.releaseCoord.SetProgress(p.ProjectName, len(results))
		h.persistReleaseLogSnapshot(logPath, body, job.ReleaseRecID, results, released, skipped, failed, p.ProjectName, updatePom, createTarget)

		workDir := projectfs.ResolveProjectAbsDir(&p, ws, h.dataDir)
		var res projectrelease.ReleaseResult
		if workDir == "" {
			res = projectrelease.ReleaseResult{
				ProjectName: p.ProjectName,
				Status:      "skipped",
				Message:     "work directory is empty",
			}
			skipped++
		} else {
			res = releaser.RunRelease(ctx, &p, projectrelease.ReleaseRequest{
				WorkDir:            workDir,
				ReleaseBranch:      body.ReleaseBranch,
				Tag:                body.TagBranch,
				ReleaseNote:        strings.TrimSpace(body.ReleaseNote),
				ReleaseDate:        body.ReleaseDate,
				AllowRepeat:        allowRepeat,
				UpdatePomVersion:   updatePom,
				CreateTargetBranch: createTarget,
				TargetBranch:       body.TargetBranch,
			})
			switch res.Status {
			case "ok":
				released++
			case "skipped":
				skipped++
			default:
				failed++
			}
		}
		results = append(results, res)
		h.releaseCoord.AppendResult(res, released, skipped, failed)
	}
	h.releaseCoord.SetProgress("", len(projects))
	h.releaseCoord.FinishResult(released, skipped, failed, results)

	fin := time.Now().UTC()
	status := repository.ResolveReleaseStatus(released, skipped, failed)
	summary := "released=" + strconv.Itoa(released) + " skipped=" + strconv.Itoa(skipped) + " failed=" + strconv.Itoa(failed)
	logText := h.formatPersistedReleaseLog(body, results, released, skipped, failed, updatePom, createTarget, true, "")
	_ = os.WriteFile(logPath, []byte(logText), 0o644)
	_ = h.releases.Finish(job.ReleaseRecID, status, summary, logPath, released, skipped, failed, &fin)

	h.audit.Record(r.Context(), r, audit.Entry{
		UserRecID: auditUserRecID,
		Action:    audit.ActionUpdate, Module: audit.ModuleProject, TargetType: "workspace",
		TargetID: ws.RecID, TargetName: ws.WorkspaceName,
		Summary: "工作空间自动发版 " + body.TagBranch,
		Detail: "branch=" + body.ReleaseBranch + " target=" + body.TargetBranch +
			" create_target=" + strconv.FormatBool(createTarget) +
			" released=" + strconv.Itoa(released) +
			" skipped=" + strconv.Itoa(skipped) + " failed=" + strconv.Itoa(failed),
		Success: failed == 0,
	})
}

func (h *Handler) persistReleaseLogSnapshot(
	logPath string,
	body batchAutoReleasePayload,
	releaseRecID string,
	results []projectrelease.ReleaseResult,
	released, skipped, failed int,
	currentProject string,
	updatePom, createTarget bool,
) {
	if releaseRecID == "" || logPath == "" {
		return
	}
	logText := h.formatPersistedReleaseLog(body, results, released, skipped, failed, updatePom, createTarget, false, currentProject)
	_ = os.WriteFile(logPath, []byte(logText), 0o644)
}

func (h *Handler) formatPersistedReleaseLog(
	body batchAutoReleasePayload,
	results []projectrelease.ReleaseResult,
	released, skipped, failed int,
	updatePom, createTarget, final bool,
	currentProject string,
) string {
	target := "—"
	if createTarget {
		target = strings.TrimSpace(body.TargetBranch)
		if target == "" {
			target = "—"
		}
	}
	pomOn := "是"
	pomOff := "否"
	if !updatePom {
		pomOn, pomOff = pomOff, pomOn
	}
	note := strings.TrimSpace(body.ReleaseNote)
	if note == "" {
		note = body.TagBranch + "自动发版说明"
	}
	return projectrelease.FormatBatchReleaseLog(projectrelease.BatchLogParams{
		StartLabel:        "开始发版…",
		DoneLabel:         "发版结束",
		ProgressLabel:     "发版进度（实时）",
		ParamsLine:        "发版分支: " + body.ReleaseBranch + " · 版本 Tag: " + body.TagBranch + " · 目标分支: " + target,
		NoteLine:          "发版说明: " + note,
		DateLine:          "发布时间: " + body.ReleaseDate,
		PomLine:           "更新 POM: " + map[bool]string{true: pomOn, false: pomOff}[updatePom],
		SummaryLine:       "汇总 — 成功 " + strconv.Itoa(released) + "，跳过 " + strconv.Itoa(skipped) + "，失败 " + strconv.Itoa(failed) + "（" + body.TagBranch + "）",
		NoStepsLabel:      "（无步骤明细，请确认后端已重新编译并重启）",
		NoMessageLabel:    "（无错误详情）",
		InProgressLabel:   "正在处理：%s",
		InProgressProject: currentProject,
		Final:             final,
		Running:           !final,
		Released:          released,
		Skipped:           skipped,
		Failed:            failed,
		Results:           results,
	})
}
