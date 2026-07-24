package workspace

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"auto-pack/backend/internal/httpx"
	"auto-pack/backend/pkg/packrun"
)

type workspaceSourceSyncStarter interface {
	StartWorkspaceSourceSync(workspaceRecID, workspaceName string) error
	WorkspaceSourceSyncStatus(workspaceRecID string) packrun.WorkspaceSourceSyncJob
	WorkspaceSourceSyncCapacity() (running, max int, names []string)
}

// syncSources handles POST /workspaces/{recID}/sync-sources (async; poll GET .../sync-sources/status).
func (h *Handler) syncSources(w http.ResponseWriter, r *http.Request) {
	wsRecID := strings.TrimSpace(r.PathValue("recID"))
	if wsRecID == "" {
		httpx.WriteError(w, http.StatusBadRequest, "workspace rec_id required")
		return
	}
	ws, err := h.repo.FindByRecID(wsRecID)
	if err != nil {
		httpx.WriteError(w, http.StatusNotFound, "workspace not found")
		return
	}
	starter, ok := h.releaseGuard.(workspaceSourceSyncStarter)
	if !ok {
		httpx.WriteError(w, http.StatusInternalServerError, "build runner not available")
		return
	}
	if err := starter.StartWorkspaceSourceSync(wsRecID, ws.WorkspaceName); err != nil {
		if errors.Is(err, packrun.ErrWorkspaceSyncCapacity) {
			running, max, names := starter.WorkspaceSourceSyncCapacity()
			msg := "源码预同步并发已满（" + strconv.Itoa(running) + "/" + strconv.Itoa(max) + " 个工作空间"
			if len(names) > 0 {
				msg += "：" + strings.Join(names, "、")
			}
			msg += "），请稍后重试"
			httpx.WriteError(w, http.StatusConflict, msg)
			return
		}
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	httpx.WriteJSON(w, http.StatusAccepted, map[string]string{
		"status": packrun.SyncJobStatusRunning,
	})
}

// syncSourcesStatus handles GET /workspaces/{recID}/sync-sources/status
func (h *Handler) syncSourcesStatus(w http.ResponseWriter, r *http.Request) {
	wsRecID := strings.TrimSpace(r.PathValue("recID"))
	if wsRecID == "" {
		httpx.WriteError(w, http.StatusBadRequest, "workspace rec_id required")
		return
	}
	if _, err := h.repo.FindByRecID(wsRecID); err != nil {
		httpx.WriteError(w, http.StatusNotFound, "workspace not found")
		return
	}
	starter, ok := h.releaseGuard.(workspaceSourceSyncStarter)
	if !ok {
		httpx.WriteError(w, http.StatusInternalServerError, "build runner not available")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, starter.WorkspaceSourceSyncStatus(wsRecID))
}
