package project

import (
	"encoding/json"
	"errors"
	"net/http"

	"auto-pack/backend/internal/httpx"
	"auto-pack/backend/internal/repository"
	"auto-pack/backend/internal/web/middleware"
	"auto-pack/backend/pkg/audit"
	"auto-pack/backend/pkg/projectcfg"
	"auto-pack/backend/pkg/projectrelease"
)

type pushLocalSourcePayload struct {
	CommitMessage string `json:"commit_message"`
	AllowRepeat   *bool  `json:"allow_repeat"`
}

type pushLocalSourceResponse struct {
	Steps []string `json:"steps"`
}

func (h *Handler) pushLocalSource(w http.ResponseWriter, r *http.Request) {
	recID := r.PathValue("recID")
	project, err := h.projects.FindByRecID(recID)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	if projectcfg.UsesGitSource(project) {
		httpx.WriteError(w, http.StatusBadRequest, "Git source projects sync from remote; use release or pom update to push")
		return
	}
	if !projectcfg.HasRemotePush(project) {
		httpx.WriteError(w, http.StatusBadRequest, "remote repository URL is not configured")
		return
	}
	ws, err := h.workspaces.FindByRecID(project.WorkspaceRecID)
	if err != nil {
		if errors.Is(err, repository.ErrWorkspaceNotFound) {
			httpx.WriteError(w, http.StatusNotFound, err.Error())
			return
		}
		httpx.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	var body pushLocalSourcePayload
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&body)
	}
	allowRepeat := true
	if body.AllowRepeat != nil {
		allowRepeat = *body.AllowRepeat
	}
	auditUserRecID, _ := middleware.RecIDFromContext(r.Context())
	release := h.runner.LockProjectBuild(project.RecID)
	git := &projectrelease.GitCommitter{EnvRepo: h.env}
	steps, err := git.PushLocalSource(r.Context(), project, ws, h.dataDir, body.CommitMessage, allowRepeat)
	release()
	if err != nil {
		h.audit.Record(r.Context(), r, audit.Entry{
			UserRecID: auditUserRecID,
			Action:    audit.ActionUpdate, Module: audit.ModuleProject, TargetType: "project",
			TargetID: project.RecID, TargetName: project.ProjectName,
			Summary: "推送本地源码失败 " + project.ProjectName,
			Detail:  err.Error(), Success: false,
		})
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	h.audit.Record(r.Context(), r, audit.Entry{
		UserRecID: auditUserRecID,
		Action:    audit.ActionUpdate, Module: audit.ModuleProject, TargetType: "project",
		TargetID: project.RecID, TargetName: project.ProjectName,
		Summary: "推送本地源码 " + project.ProjectName, Success: true,
	})
	httpx.WriteJSON(w, http.StatusOK, pushLocalSourceResponse{Steps: steps})
}
