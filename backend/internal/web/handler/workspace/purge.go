package workspace

import (
	"net/http"
	"strconv"

	"auto-pack/backend/internal/httpx"
	"auto-pack/backend/internal/repository"
	"auto-pack/backend/pkg/audit"
)

type purgeWorkspaceResponse struct {
	DeletedProjects int `json:"deleted_projects"`
}

func (h *Handler) purge(w http.ResponseWriter, r *http.Request) {
	recID := r.PathValue("recID")
	if recID == repository.DefaultWorkspaceRecID {
		httpx.WriteError(w, http.StatusBadRequest, "cannot delete default workspace")
		return
	}
	ws, err := h.repo.FindByRecID(recID)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	if ws.IsDefault == 1 {
		httpx.WriteError(w, http.StatusBadRequest, "cannot delete default workspace")
		return
	}
	if h.releaseGuard != nil && h.releaseGuard.WorkspaceReleaseActive(recID) {
		httpx.WriteError(w, http.StatusConflict, "auto release is running for this workspace")
		return
	}
	if runningRelease, err := h.releases.FindRunningByWorkspace(recID); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	} else if runningRelease != nil {
		httpx.WriteError(w, http.StatusConflict, "auto release is running for this workspace")
		return
	}
	if runningBuild, err := h.builds.FindRunningByWorkspace(recID); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	} else if runningBuild != nil {
		httpx.WriteError(w, http.StatusConflict, "cannot purge workspace while a build is running")
		return
	}
	projectCount, err := h.repo.CountProjects(recID)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := h.builds.DeleteByWorkspace(recID); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if _, err := h.projects.DeleteByWorkspace(recID); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := h.releases.DeleteByWorkspace(recID); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := h.repo.DeleteUnchecked(recID); err != nil {
		writeRepoError(w, err)
		return
	}
	if h.scheduler != nil {
		_ = h.scheduler.ReloadBuildScheduler()
	}
	h.audit.Record(r.Context(), r, audit.Entry{
		Action: audit.ActionDelete, Module: audit.ModuleWorkspace, TargetType: "workspace",
		TargetID: recID, TargetName: ws.WorkspaceName,
		Summary: "删除工作空间及其工程 " + ws.WorkspaceName,
		Detail:  "projects_deleted=" + strconv.Itoa(projectCount),
		Success: true,
	})
	httpx.WriteJSON(w, http.StatusOK, purgeWorkspaceResponse{DeletedProjects: projectCount})
}
