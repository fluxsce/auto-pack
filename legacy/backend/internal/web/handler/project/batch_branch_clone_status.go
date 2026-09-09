package project

import (
	"net/http"
	"strings"

	"auto-pack/backend/internal/httpx"
	"auto-pack/backend/internal/repository"
	"auto-pack/backend/pkg/projectrelease"
)

func (h *Handler) batchBranchCloneStatus(w http.ResponseWriter, r *http.Request) {
	wsID := strings.TrimSpace(r.URL.Query().Get("workspace_rec_id"))
	if wsID == "" {
		httpx.WriteError(w, http.StatusBadRequest, "workspace_rec_id is required")
		return
	}
	st := h.enrichBatchBranchCloneStatus(wsID, h.branchCloneCoord.Status(wsID))
	httpx.WriteJSON(w, http.StatusOK, st)
}

func (h *Handler) enrichBatchBranchCloneStatus(wsID string, st projectrelease.BatchBranchCloneStatus) projectrelease.BatchBranchCloneStatus {
	wsID = strings.TrimSpace(wsID)
	if st.Status == "idle" && wsID != "" {
		if rec, err := h.branchClones.FindRunningByWorkspace(wsID); err == nil && rec != nil {
			st = statusFromBranchCloneRecord(rec)
		}
	}
	batchID := strings.TrimSpace(st.BatchRecID)
	if batchID == "" && wsID != "" {
		if rec, err := h.branchClones.FindRunningByWorkspace(wsID); err == nil && rec != nil {
			if st.Status == "idle" {
				st = statusFromBranchCloneRecord(rec)
			}
		}
	}
	return st
}

func statusFromBranchCloneRecord(rec *repository.PackWorkspaceBranchClone) projectrelease.BatchBranchCloneStatus {
	done := rec.Cloned + rec.Skipped + rec.Failed
	return projectrelease.BatchBranchCloneStatus{BatchBranchCloneJob: projectrelease.BatchBranchCloneJob{
		BatchRecID:     rec.RecID,
		WorkspaceRecID: rec.WorkspaceRecID,
		Status:         projectrelease.JobStatusRunning,
		SourceBranch:   rec.SourceBranch,
		TargetBranch:   rec.TargetBranch,
		Cloned:         rec.Cloned,
		Skipped:        rec.Skipped,
		Failed:         rec.Failed,
		DoneCount:      done,
		TotalCount:     done,
	}}
}
