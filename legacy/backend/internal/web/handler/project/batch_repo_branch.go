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
)

type batchRepoBranchPayload struct {
	WorkspaceRecID string `json:"workspace_rec_id"`
	RepoBranch     string `json:"repo_branch"`
}

type batchRepoBranchResponse struct {
	Updated int64 `json:"updated"`
}

func (h *Handler) batchRepoBranch(w http.ResponseWriter, r *http.Request) {
	var body batchRepoBranchPayload
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	wsID := strings.TrimSpace(body.WorkspaceRecID)
	if wsID == "" {
		httpx.WriteError(w, http.StatusBadRequest, "workspace_rec_id is required")
		return
	}
	branch := strings.TrimSpace(body.RepoBranch)
	if branch == "" {
		httpx.WriteError(w, http.StatusBadRequest, "repo_branch is required")
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
	n, err := h.projects.UpdateRepoBranchByWorkspace(wsID, branch)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	_ = h.runner.Scheduler().Reload()
	h.audit.Record(r.Context(), r, audit.Entry{
		Action: audit.ActionUpdate, Module: audit.ModuleProject, TargetType: "workspace",
		TargetID: ws.RecID, TargetName: ws.WorkspaceName,
		Summary: "批量修改 Git 分支为 " + branch,
		Detail:  "projects_updated=" + strconv.FormatInt(n, 10),
		Success: true,
	})
	httpx.WriteJSON(w, http.StatusOK, batchRepoBranchResponse{Updated: n})
}
