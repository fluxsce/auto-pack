package project

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"auto-pack/backend/internal/httpx"
	"auto-pack/backend/internal/repository"
	"auto-pack/backend/pkg/packrun"
)

func (h *Handler) listBatchPomVersionHistory(w http.ResponseWriter, r *http.Request) {
	wsID := strings.TrimSpace(r.URL.Query().Get("workspace_rec_id"))
	if wsID == "" {
		httpx.WriteError(w, http.StatusBadRequest, "workspace_rec_id is required")
		return
	}
	if _, err := h.workspaces.FindByRecID(wsID); err != nil {
		writeWorkspaceLoadError(w, err)
		return
	}
	limit := 30
	if q := strings.TrimSpace(r.URL.Query().Get("limit")); q != "" {
		if n, err := strconv.Atoi(q); err == nil {
			limit = n
		}
	}
	items, err := h.pomVersions.ListByWorkspace(wsID, limit)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) getBatchPomVersionLog(w http.ResponseWriter, r *http.Request) {
	batchID := strings.TrimSpace(r.PathValue("batchID"))
	if batchID == "" {
		httpx.WriteError(w, http.StatusBadRequest, "batch id is required")
		return
	}
	wsID := strings.TrimSpace(r.URL.Query().Get("workspace_rec_id"))
	if wsID == "" {
		httpx.WriteError(w, http.StatusBadRequest, "workspace_rec_id is required")
		return
	}
	item, err := h.pomVersions.FindByRecID(batchID)
	if err != nil {
		writeWorkspacePomVersionError(w, err)
		return
	}
	if item.WorkspaceRecID != wsID {
		httpx.WriteError(w, http.StatusNotFound, repository.ErrWorkspacePomVersionNotFound.Error())
		return
	}
	offset := int64(0)
	if q := strings.TrimSpace(r.URL.Query().Get("offset")); q != "" {
		if n, err := strconv.ParseInt(q, 10, 64); err == nil {
			offset = n
		}
	}
	content, total, err := packrun.ReadLogTail(item.LogPath, offset, 512*1024)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	complete := item.Status != repository.PomVersionStatusRunning
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"content":  content,
		"offset":   offset,
		"size":     len(content),
		"total":    total,
		"complete": complete,
		"status":   item.Status,
		"batch":    item,
	})
}

func writeWorkspacePomVersionError(w http.ResponseWriter, err error) {
	if errors.Is(err, repository.ErrWorkspacePomVersionNotFound) {
		httpx.WriteError(w, http.StatusNotFound, err.Error())
		return
	}
	httpx.WriteError(w, http.StatusInternalServerError, err.Error())
}
