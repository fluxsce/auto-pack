package project

import (
	"strings"

	"auto-pack/backend/internal/repository"
	"auto-pack/backend/pkg/projectrelease"
)

func (h *Handler) enrichBatchReleaseStatus(wsID string, st projectrelease.BatchReleaseStatus) projectrelease.BatchReleaseStatus {
	wsID = strings.TrimSpace(wsID)
	if st.Status == "idle" && wsID != "" {
		if rec, err := h.releases.FindRunningByWorkspace(wsID); err == nil && rec != nil {
			st = statusFromReleaseRecord(rec)
		}
	}
	releaseID := strings.TrimSpace(st.ReleaseRecID)
	if releaseID == "" && wsID != "" {
		if rec, err := h.releases.FindRunningByWorkspace(wsID); err == nil && rec != nil {
			releaseID = rec.RecID
			if st.Status == "idle" {
				st = statusFromReleaseRecord(rec)
			}
		}
	}
	if st.Status == projectrelease.JobStatusRunning && releaseID != "" {
		if rec, err := h.releases.FindByRecID(releaseID); err == nil {
			if rec.Status == repository.ReleaseStatusRunning {
				st.PostOptions = parseStoredPostOptions(rec.PostOptionsJSON)
			}
		}
	}
	return st
}

func statusFromReleaseRecord(rec *repository.PackWorkspaceRelease) projectrelease.BatchReleaseStatus {
	done := rec.Released + rec.Skipped + rec.Failed
	return projectrelease.BatchReleaseStatus{BatchReleaseJob: projectrelease.BatchReleaseJob{
		ReleaseRecID:   rec.RecID,
		WorkspaceRecID: rec.WorkspaceRecID,
		Status:         projectrelease.JobStatusRunning,
		Tag:            rec.TagBranch,
		ReleaseBranch:  rec.ReleaseBranch,
		Released:       rec.Released,
		Skipped:        rec.Skipped,
		Failed:         rec.Failed,
		DoneCount:      done,
		TotalCount:     done,
		PostOptions:    parseStoredPostOptions(rec.PostOptionsJSON),
	}}
}

func parseStoredPostOptions(raw string) *projectrelease.PostOptions {
	opts, err := projectrelease.ParsePostOptionsJSON(raw)
	if err != nil {
		return nil
	}
	return opts
}

func (h *Handler) resolveActiveReleaseRecID(wsID string) (string, error) {
	wsID = strings.TrimSpace(wsID)
	if wsID == "" {
		return "", errNoRunningRelease
	}
	st := h.releaseCoord.Status(wsID)
	if !st.Foreign && st.Status == projectrelease.JobStatusRunning &&
		strings.TrimSpace(st.ReleaseRecID) != "" {
		return st.ReleaseRecID, nil
	}
	rec, err := h.releases.FindRunningByWorkspace(wsID)
	if err != nil {
		return "", err
	}
	if rec == nil {
		return "", errNoRunningRelease
	}
	return rec.RecID, nil
}
