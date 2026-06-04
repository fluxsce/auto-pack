package project

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"auto-pack/backend/internal/httpx"
	"auto-pack/backend/internal/repository"
	"auto-pack/backend/pkg/audit"
	"auto-pack/backend/pkg/envtypes"
	"auto-pack/backend/pkg/projectfs"
	"auto-pack/backend/pkg/workspaceftppush"
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

	result, err := workspaceftppush.Push(workspaceftppush.Request{
		LocalBase:    base,
		RelativePath: rel,
		FTPConfig:    ftpCfg,
		PushPath:     body.PushPath,
		BackupPath:   body.BackupPath,
		Pusher:       body.Pusher,
		TaskRef:      body.TaskRef,
		Detail:       body.Detail,
	})
	if err != nil {
		switch {
		case errors.Is(err, workspaceftppush.ErrNotFound):
			httpx.WriteError(w, http.StatusNotFound, err.Error())
		case errors.Is(err, workspaceftppush.ErrPathRequired),
			errors.Is(err, workspaceftppush.ErrPushPathRequired),
			errors.Is(err, workspaceftppush.ErrPusherRequired),
			errors.Is(err, workspaceftppush.ErrTaskRefRequired):
			httpx.WriteError(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, projectfs.ErrPathTraversal):
			httpx.WriteError(w, http.StatusBadRequest, err.Error())
		default:
			httpx.WriteError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	h.audit.Record(r.Context(), r, audit.Entry{
		Action: audit.ActionUpdate, Module: audit.ModuleProject, TargetType: "project",
		TargetID: projectRecID, TargetName: project.ProjectName,
		Summary: "FTP 远程推送 " + rel + " → " + result.UpdatePath,
		Detail:  result.InfoContent,
		Success: true,
	})
	httpx.WriteJSON(w, http.StatusOK, result)
}
