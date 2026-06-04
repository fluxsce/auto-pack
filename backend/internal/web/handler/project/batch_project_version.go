package project

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"auto-pack/backend/internal/httpx"
	"auto-pack/backend/internal/repository"
	"auto-pack/backend/pkg/audit"
	"auto-pack/backend/pkg/projectcfg"
)

type batchProjectVersionPayload struct {
	WorkspaceRecID string `json:"workspace_rec_id"`
	ProjectVersion string `json:"project_version"`
}

type batchProjectVersionResponse struct {
	Updated int64 `json:"updated"`
}

func (h *Handler) batchProjectVersion(w http.ResponseWriter, r *http.Request) {
	var body batchProjectVersionPayload
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	wsID := strings.TrimSpace(body.WorkspaceRecID)
	if wsID == "" {
		httpx.WriteError(w, http.StatusBadRequest, "workspace_rec_id is required")
		return
	}
	version := strings.TrimSpace(body.ProjectVersion)
	if version == "" {
		httpx.WriteError(w, http.StatusBadRequest, "project_version is required")
		return
	}
	if err := projectcfg.ValidateVersion(version); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	ws, err := h.workspaces.FindByRecID(wsID)
	if err != nil {
		if errors.Is(err, repository.ErrWorkspaceNotFound) {
			httpx.WriteError(w, http.StatusNotFound, err.Error())
			return
		}
		httpx.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	dup, err := h.projects.HasDuplicateProjectNamesInWorkspace(wsID)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if dup {
		httpx.WriteError(w, http.StatusBadRequest,
			"workspace has multiple projects with the same name; resolve duplicates before batch version update")
		return
	}
	n, err := h.projects.UpdateVersionByWorkspace(wsID, version)
	if err != nil {
		if errors.Is(err, repository.ErrProjectVersionExists) {
			httpx.WriteError(w, http.StatusConflict, err.Error())
			return
		}
		httpx.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	h.audit.Record(r.Context(), r, audit.Entry{
		Action: audit.ActionUpdate, Module: audit.ModuleProject, TargetType: "workspace",
		TargetID: ws.RecID, TargetName: ws.WorkspaceName,
		Summary: "批量修改工程版本为 " + version,
		Detail:  "projects_updated=" + strconv.FormatInt(n, 10),
		Success: true,
	})
	httpx.WriteJSON(w, http.StatusOK, batchProjectVersionResponse{Updated: n})
}
