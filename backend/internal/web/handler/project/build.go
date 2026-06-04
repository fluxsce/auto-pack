package project

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"auto-pack/backend/internal/httpx"
	"auto-pack/backend/internal/repository"
	"auto-pack/backend/pkg/audit"
	"auto-pack/backend/pkg/packrun"
	"auto-pack/backend/pkg/projectcfg"
	"auto-pack/backend/pkg/projectfs"
)

func (h *Handler) listBuilds(w http.ResponseWriter, r *http.Request) {
	projectRecID := r.PathValue("recID")
	project, err := h.projects.FindByRecID(projectRecID)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	if cfg, err := projectcfg.ParseConfigJSON(project.ConfigJSON); err == nil {
		_, _ = h.runner.PruneExpiredBuilds(projectRecID, cfg.LogRetentionDays)
	}
	limit := 30
	if q := strings.TrimSpace(r.URL.Query().Get("limit")); q != "" {
		if n, err := strconv.Atoi(q); err == nil {
			limit = n
		}
	}
	items, err := h.builds.ListByProject(projectRecID, limit)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) triggerBuild(w http.ResponseWriter, r *http.Request) {
	projectRecID := r.PathValue("recID")
	mode := packrun.TriggerManual
	if q := strings.TrimSpace(r.URL.Query().Get("trigger")); q != "" {
		mode = packrun.ParseTriggerModeQuery(q)
	}
	project, _ := h.projects.FindByRecID(projectRecID)
	build, err := h.runner.Trigger(projectRecID, mode)
	if err != nil {
		if errors.Is(err, repository.ErrProjectNotFound) {
			httpx.WriteError(w, http.StatusNotFound, err.Error())
			return
		}
		if errors.Is(err, repository.ErrBuildRunning) {
			httpx.WriteError(w, http.StatusConflict, err.Error())
			return
		}
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	name := projectRecID
	if project != nil {
		name = project.ProjectName
	}
	h.audit.Record(r.Context(), r, audit.Entry{
		Action: audit.ActionBuild, Module: audit.ModuleBuild, TargetType: "project",
		TargetID: projectRecID, TargetName: name,
		Summary: "触发构建 #" + strconv.Itoa(build.BuildNum) + " " + name, Success: true,
	})
	httpx.WriteJSON(w, http.StatusAccepted, build)
}

func (h *Handler) getBuild(w http.ResponseWriter, r *http.Request) {
	projectRecID := r.PathValue("recID")
	buildRecID := r.PathValue("buildID")
	build, err := h.builds.FindByRecID(buildRecID)
	if err != nil {
		writeBuildError(w, err)
		return
	}
	if build.ProjectRecID != projectRecID {
		httpx.WriteError(w, http.StatusNotFound, repository.ErrBuildNotFound.Error())
		return
	}
	httpx.WriteJSON(w, http.StatusOK, build)
}

func (h *Handler) getBuildLog(w http.ResponseWriter, r *http.Request) {
	projectRecID := r.PathValue("recID")
	buildRecID := r.PathValue("buildID")
	build, err := h.builds.FindByRecID(buildRecID)
	if err != nil {
		writeBuildError(w, err)
		return
	}
	if build.ProjectRecID != projectRecID {
		httpx.WriteError(w, http.StatusNotFound, repository.ErrBuildNotFound.Error())
		return
	}
	offset := int64(0)
	if q := strings.TrimSpace(r.URL.Query().Get("offset")); q != "" {
		if n, err := strconv.ParseInt(q, 10, 64); err == nil {
			offset = n
		}
	}
	content, total, err := packrun.ReadLogTail(build.LogPath, offset, 256*1024)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	complete := build.Status != repository.BuildStatusPending && build.Status != repository.BuildStatusRunning
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"content":  content,
		"offset":   offset,
		"size":     len(content),
		"total":    total,
		"complete": complete,
		"status":   build.Status,
		"build":    build,
	})
}

func (h *Handler) deleteWorkspace(w http.ResponseWriter, r *http.Request) {
	projectRecID := r.PathValue("recID")
	project, err := h.projects.FindByRecID(projectRecID)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	running, err := h.builds.FindRunningByProject(projectRecID)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if running != nil {
		httpx.WriteError(w, http.StatusConflict, "cannot delete workspace while a build is running")
		return
	}
	_, _, base, err := h.resolveProjectDir(projectRecID)
	if err != nil {
		writeProjectDirError(w, err)
		return
	}
	rel := strings.TrimSpace(r.URL.Query().Get("path"))
	if rel == "" {
		rel = "."
	}
	if err := projectfs.RemovePath(base, rel); err != nil {
		writeBrowseError(w, err)
		return
	}
	summary := "删除工程目录 " + project.ProjectName
	if rel != "." {
		summary = "删除工程目录项 " + project.ProjectName + ": " + rel
	}
	h.audit.Record(r.Context(), r, audit.Entry{
		Action: audit.ActionUpdate, Module: audit.ModuleProject, TargetType: "project",
		TargetID: projectRecID, TargetName: project.ProjectName,
		Summary: summary, Success: true,
	})
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"message": "deleted"})
}

func (h *Handler) browseWorkspace(w http.ResponseWriter, r *http.Request) {
	projectRecID := r.PathValue("recID")
	_, _, base, err := h.resolveProjectDir(projectRecID)
	if err != nil {
		writeProjectDirError(w, err)
		return
	}
	rel := strings.TrimSpace(r.URL.Query().Get("path"))
	listing, err := projectfs.ListDir(base, rel)
	if err != nil {
		writeBrowseError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, listing)
}

func (h *Handler) readWorkspaceFile(w http.ResponseWriter, r *http.Request) {
	projectRecID := r.PathValue("recID")
	_, _, base, err := h.resolveProjectDir(projectRecID)
	if err != nil {
		writeProjectDirError(w, err)
		return
	}
	rel := strings.TrimSpace(r.URL.Query().Get("path"))
	if rel == "" {
		httpx.WriteError(w, http.StatusBadRequest, "path is required")
		return
	}
	content, err := projectfs.ReadTextFile(base, rel)
	if err != nil {
		writeBrowseError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, content)
}

func (h *Handler) downloadWorkspaceEntry(w http.ResponseWriter, r *http.Request) {
	projectRecID := r.PathValue("recID")
	_, _, base, err := h.resolveProjectDir(projectRecID)
	if err != nil {
		writeProjectDirError(w, err)
		return
	}
	rel := strings.TrimSpace(r.URL.Query().Get("path"))
	if rel == "" {
		httpx.WriteError(w, http.StatusBadRequest, "path is required")
		return
	}
	if err := projectfs.WriteDownload(w, base, rel); err != nil {
		writeBrowseError(w, err)
		return
	}
}

func (h *Handler) resolveProjectDir(projectRecID string) (*repository.PackProject, *repository.PackWorkspace, string, error) {
	project, err := h.projects.FindByRecID(projectRecID)
	if err != nil {
		return nil, nil, "", err
	}
	ws, err := h.workspaces.FindByRecID(project.WorkspaceRecID)
	if err != nil {
		return nil, nil, "", err
	}
	base := projectfs.ResolveProjectAbsDir(project, ws, h.dataDir)
	if base == "" {
		return nil, nil, "", errors.New("project work directory is empty")
	}
	return project, ws, base, nil
}

func writeBuildError(w http.ResponseWriter, err error) {
	if errors.Is(err, repository.ErrBuildNotFound) {
		httpx.WriteError(w, http.StatusNotFound, err.Error())
		return
	}
	httpx.WriteError(w, http.StatusInternalServerError, err.Error())
}

func writeProjectDirError(w http.ResponseWriter, err error) {
	if errors.Is(err, repository.ErrProjectNotFound) {
		httpx.WriteError(w, http.StatusNotFound, err.Error())
		return
	}
	httpx.WriteError(w, http.StatusInternalServerError, err.Error())
}

func writeBrowseError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, projectfs.ErrPathTraversal):
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, projectfs.ErrNotFound):
		httpx.WriteError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, projectfs.ErrNotDirectory):
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, projectfs.ErrFileTooLarge):
		httpx.WriteError(w, http.StatusRequestEntityTooLarge, err.Error())
	case errors.Is(err, projectfs.ErrArchiveTooLarge):
		httpx.WriteError(w, http.StatusRequestEntityTooLarge, err.Error())
	default:
		httpx.WriteError(w, http.StatusInternalServerError, err.Error())
	}
}
