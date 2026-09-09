package project

import (
	"net/http"
	"strings"

	"auto-pack/backend/internal/httpx"
	"auto-pack/backend/internal/repository"
	"auto-pack/backend/pkg/projectrelease"
)

func (h *Handler) batchPomVersionStatus(w http.ResponseWriter, r *http.Request) {
	wsID := strings.TrimSpace(r.URL.Query().Get("workspace_rec_id"))
	if wsID == "" {
		httpx.WriteError(w, http.StatusBadRequest, "workspace_rec_id is required")
		return
	}
	st := h.enrichBatchPomVersionStatus(wsID, h.pomVersionCoord.Status(wsID))
	httpx.WriteJSON(w, http.StatusOK, st)
}

func (h *Handler) enrichBatchPomVersionStatus(wsID string, st projectrelease.BatchPomVersionStatus) projectrelease.BatchPomVersionStatus {
	wsID = strings.TrimSpace(wsID)
	if st.Status == "idle" && wsID != "" {
		if rec, err := h.pomVersions.FindRunningByWorkspace(wsID); err == nil && rec != nil {
			st = statusFromPomVersionRecord(rec)
		}
	}
	batchID := strings.TrimSpace(st.BatchRecID)
	if batchID == "" && wsID != "" {
		if rec, err := h.pomVersions.FindRunningByWorkspace(wsID); err == nil && rec != nil {
			batchID = rec.RecID
			if st.Status == "idle" {
				st = statusFromPomVersionRecord(rec)
			}
		}
	}
	return st
}

func statusFromPomVersionRecord(rec *repository.PackWorkspacePomVersion) projectrelease.BatchPomVersionStatus {
	done := rec.Updated + rec.Skipped + rec.Failed
	return projectrelease.BatchPomVersionStatus{BatchPomVersionJob: projectrelease.BatchPomVersionJob{
		BatchRecID:     rec.RecID,
		WorkspaceRecID: rec.WorkspaceRecID,
		Status:         projectrelease.JobStatusRunning,
		Tag:            rec.VersionTag,
		CommitMessage:  rec.CommitMessage,
		Updated:        rec.Updated,
		Skipped:        rec.Skipped,
		Failed:         rec.Failed,
		DoneCount:      done,
		TotalCount:     done,
	}}
}
