package project

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"auto-pack/backend/internal/httpx"
	"auto-pack/backend/internal/repository"
	"auto-pack/backend/pkg/audit"
	"auto-pack/backend/pkg/envtypes"
	"auto-pack/backend/pkg/workspaceftppush"

	"github.com/google/uuid"
)

type workspaceFtpPushPayload struct {
	Path        string `json:"path"`
	FTPEnvRecID string `json:"ftp_env_rec_id"`
	PushPath    string `json:"push_path"`
	BackupPath  string `json:"backup_path"`
	Pusher      string `json:"pusher"`
	TaskRef     string `json:"task_ref"`
	Detail      string `json:"detail"`
}

type workspaceFtpPushStartResponse struct {
	TaskID string `json:"task_id"`
	Status string `json:"status"`
}

func (h *Handler) workspaceFtpPush(w http.ResponseWriter, r *http.Request) {
	projectRecID := r.PathValue("recID")
	if _, err := h.projects.FindByRecID(projectRecID); err != nil {
		writeRepoError(w, err)
		return
	}
	running, err := h.builds.FindRunningByProject(projectRecID)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if running != nil {
		httpx.WriteError(w, http.StatusConflict, "cannot push while a build is running")
		return
	}
	if active := h.ftpPushCoord.activeTask(projectRecID); active != nil {
		httpx.WriteError(w, http.StatusConflict, "ftp push already in progress")
		return
	}

	var body workspaceFtpPushPayload
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	rel := strings.TrimSpace(body.Path)
	if rel == "" || rel == "." {
		httpx.WriteError(w, http.StatusBadRequest, "path is required")
		return
	}
	envID := strings.TrimSpace(body.FTPEnvRecID)
	if envID == "" {
		httpx.WriteError(w, http.StatusBadRequest, "ftp_env_rec_id is required")
		return
	}
	item, err := h.env.FindByRecID(envID)
	if err != nil {
		if errors.Is(err, repository.ErrEnvNotFound) {
			httpx.WriteError(w, http.StatusNotFound, err.Error())
			return
		}
		httpx.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if item.EnvType != envtypes.TypeFTP {
		httpx.WriteError(w, http.StatusBadRequest, "environment must be ftp")
		return
	}
	ftpCfg, err := workspaceftppush.ParseFTPConfig(item.ConfigJSON)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	project, err := h.projects.FindByRecID(projectRecID)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	_, _, base, err := h.resolveProjectDir(projectRecID)
	if err != nil {
		writeProjectDirError(w, err)
		return
	}

	taskID := uuid.New().String()
	h.ftpPushCoord.start(taskID, projectRecID, rel, body.PushPath, func(ctx context.Context, appendLog func(format string, args ...any)) (*workspaceftppush.Result, error) {
		appendLog("开始 FTP 远程推送")
		appendLog("对象：%s", rel)
		appendLog("FTP 环境：%s", item.EnvName)
		appendLog("推送路径：%s", strings.TrimSpace(body.PushPath))
		result, err := workspaceftppush.Push(workspaceftppush.Request{
			LocalBase:    base,
			RelativePath: rel,
			FTPConfig:    ftpCfg,
			PushPath:     body.PushPath,
			BackupPath:   body.BackupPath,
			Pusher:       body.Pusher,
			TaskRef:      body.TaskRef,
			Detail:       body.Detail,
			Logf:         appendLog,
		})
		if err != nil {
			appendLog("推送失败")
			appendLog("  %s", err.Error())
			h.audit.Record(r.Context(), r, audit.Entry{
				Action: audit.ActionUpdate, Module: audit.ModuleProject, TargetType: "project",
				TargetID: projectRecID, TargetName: project.ProjectName,
				Summary: "FTP 远程推送失败 " + rel,
				Detail:  err.Error(),
				Success: false,
			})
			return nil, err
		}
		appendLog("推送成功")
		appendLog("  备份路径：%s", result.BackupPath)
		appendLog("  更新包路径：%s", result.UpdatePath)
		appendLog("  说明文件：%s", result.InfoFilePath)
		h.audit.Record(r.Context(), r, audit.Entry{
			Action: audit.ActionUpdate, Module: audit.ModuleProject, TargetType: "project",
			TargetID: projectRecID, TargetName: project.ProjectName,
			Summary: "FTP 远程推送 " + rel + " → " + result.UpdatePath,
			Detail:  result.InfoContent,
			Success: true,
		})
		return &result, nil
	})
	httpx.WriteJSON(w, http.StatusAccepted, workspaceFtpPushStartResponse{
		TaskID: taskID,
		Status: "running",
	})
}
