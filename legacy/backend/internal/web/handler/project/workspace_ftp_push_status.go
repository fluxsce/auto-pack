package project

import (
	"net/http"
	"strings"

	"auto-pack/backend/internal/httpx"
)

func (h *Handler) workspaceFtpPushStatus(w http.ResponseWriter, r *http.Request) {
	projectRecID := r.PathValue("recID")
	taskID := strings.TrimSpace(r.URL.Query().Get("task_id"))
	task := h.ftpPushCoord.status(projectRecID, taskID)
	if task == nil {
		httpx.WriteJSON(w, http.StatusOK, map[string]any{
			"status":  "idle",
			"task_id": taskID,
		})
		return
	}
	httpx.WriteJSON(w, http.StatusOK, task)
}
