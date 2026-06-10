package project

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"auto-pack/backend/internal/httpx"
	"auto-pack/backend/pkg/projectrelease"
)

var errNoRunningRelease = errors.New("no active auto release for this workspace")

type batchAutoReleasePostOptionsPayload struct {
	WorkspaceRecID string                      `json:"workspace_rec_id"`
	PostOptions    *projectrelease.PostOptions `json:"post_options"`
}

func (h *Handler) batchAutoReleasePostOptions(w http.ResponseWriter, r *http.Request) {
	var body batchAutoReleasePostOptionsPayload
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	wsID := strings.TrimSpace(body.WorkspaceRecID)
	if wsID == "" {
		httpx.WriteError(w, http.StatusBadRequest, "workspace_rec_id is required")
		return
	}
	if body.PostOptions == nil {
		httpx.WriteError(w, http.StatusBadRequest, "post_options is required")
		return
	}
	releaseID, err := h.resolveActiveReleaseRecID(wsID)
	if err != nil {
		if errors.Is(err, errNoRunningRelease) {
			httpx.WriteError(w, http.StatusConflict, "当前工作空间没有进行中的自动发版")
			return
		}
		httpx.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	raw, err := projectrelease.MarshalPostOptions(body.PostOptions)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := h.releases.UpdatePostOptions(releaseID, raw); err != nil {
		writeWorkspaceReleaseError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]string{
		"release_rec_id": releaseID,
		"status":         "ok",
	})
}
