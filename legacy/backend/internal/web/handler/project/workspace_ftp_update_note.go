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
	"auto-pack/backend/pkg/workspaceftppush"
)

type workspaceFtpUpdateNotePayload struct {
	FTPEnvRecID string                          `json:"ftp_env_rec_id"`
	PushPath    string                          `json:"push_path"`
	Pusher      string                          `json:"pusher"`
	TaskRef     string                          `json:"task_ref"`
	Detail      string                          `json:"detail"`
	Paths       []workspaceftppush.NotePathPair `json:"paths"`
}

func (h *Handler) workspaceFtpUpdateNote(w http.ResponseWriter, r *http.Request) {
	projectRecID := r.PathValue("recID")
	project, err := h.projects.FindByRecID(projectRecID)
	if err != nil {
		writeRepoError(w, err)
		return
	}

	var body workspaceFtpUpdateNotePayload
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid request body")
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

	result, err := workspaceftppush.AppendUpdateNote(workspaceftppush.AppendUpdateNoteRequest{
		FTPConfig: ftpCfg,
		PushPath:  body.PushPath,
		Pusher:    body.Pusher,
		TaskRef:   body.TaskRef,
		Detail:    body.Detail,
		Paths:     body.Paths,
	})
	if err != nil {
		h.audit.Record(r.Context(), r, audit.Entry{
			Action: audit.ActionUpdate, Module: audit.ModuleProject, TargetType: "project",
			TargetID: projectRecID, TargetName: project.ProjectName,
			Summary: "FTP 合并写入 updateNote 失败",
			Detail:  err.Error(),
			Success: false,
		})
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	h.audit.Record(r.Context(), r, audit.Entry{
		Action: audit.ActionUpdate, Module: audit.ModuleProject, TargetType: "project",
		TargetID: projectRecID, TargetName: project.ProjectName,
		Summary: "FTP 合并写入 updateNote",
		Detail:  result.InfoContent,
		Success: true,
	})
	httpx.WriteJSON(w, http.StatusOK, result)
}
