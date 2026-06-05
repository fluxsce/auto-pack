package project

import (
	"errors"
	"net/http"
	"strings"

	"auto-pack/backend/internal/httpx"
	"auto-pack/backend/internal/repository"
	"auto-pack/backend/pkg/envtypes"
	"auto-pack/backend/pkg/workspaceftppush"
)

func (h *Handler) workspaceFtpRemoteNote(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		httpx.WriteError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	projectRecID := r.PathValue("recID")
	if _, err := h.projects.FindByRecID(projectRecID); err != nil {
		writeRepoError(w, err)
		return
	}

	envID := strings.TrimSpace(r.URL.Query().Get("ftp_env_rec_id"))
	if envID == "" {
		httpx.WriteError(w, http.StatusBadRequest, "ftp_env_rec_id is required")
		return
	}
	pushPath := strings.TrimSpace(r.URL.Query().Get("push_path"))
	if pushPath == "" {
		httpx.WriteError(w, http.StatusBadRequest, "push_path is required")
		return
	}
	uploadName := strings.TrimSpace(r.URL.Query().Get("file_name"))

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

	note, err := workspaceftppush.FetchRemoteUpdateNote(ftpCfg, pushPath, uploadName)
	if err != nil {
		if errors.Is(err, workspaceftppush.ErrPushPathRequired) {
			httpx.WriteError(w, http.StatusBadRequest, err.Error())
			return
		}
		httpx.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	httpx.WriteJSON(w, http.StatusOK, note)
}
