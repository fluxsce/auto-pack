package project

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"auto-pack/backend/internal/httpx"
	"auto-pack/backend/internal/repository"
	"auto-pack/backend/pkg/audit"
	"auto-pack/backend/pkg/projectcfg"
)

type batchPostBuildEmailPayload struct {
	WorkspaceRecID string `json:"workspace_rec_id"`
	Enabled        bool   `json:"enabled"`
	When           string `json:"when"`
	EnvRecID       string `json:"env_rec_id"`
	Recipients     string `json:"recipients"`
	Subject        string `json:"subject"`
	Body           string `json:"body"`
	Name           string `json:"name"`
}

type batchPostBuildEmailResponse struct {
	Updated int `json:"updated"`
}

func (h *Handler) batchPostBuildEmail(w http.ResponseWriter, r *http.Request) {
	var body batchPostBuildEmailPayload
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	wsID := strings.TrimSpace(body.WorkspaceRecID)
	if wsID == "" {
		httpx.WriteError(w, http.StatusBadRequest, "workspace_rec_id is required")
		return
	}
	opts := projectcfg.EmailNotifyBulkOpts{
		Enabled:    body.Enabled,
		When:       body.When,
		EnvRecID:   body.EnvRecID,
		Recipients: body.Recipients,
		Subject:    body.Subject,
		Body:       body.Body,
		Name:       body.Name,
	}
	if err := projectcfg.ValidateEmailNotifyBulk(opts); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	ws, err := h.workspaces.FindByRecID(wsID)
	if err != nil {
		if errors.Is(err, repository.ErrWorkspaceNotFound) {
			httpx.WriteError(w, http.StatusNotFound, err.Error())
			return
		}
		httpx.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	projects, err := h.projects.List(wsID, "")
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	updated := 0
	for _, p := range projects {
		cfg, err := projectcfg.ParseConfigJSON(p.ConfigJSON)
		if err != nil {
			httpx.WriteError(w, http.StatusBadRequest, "project "+p.ProjectName+": invalid config_json")
			return
		}
		projectcfg.ApplyEmailNotifyToConfig(&cfg, opts)
		if err := projectcfg.ValidateBuildTriggers(p.RecID, cfg.BuildTriggers); err != nil {
			httpx.WriteError(w, http.StatusBadRequest, err.Error())
			return
		}
		normalized, err := projectcfg.NormalizeConfigJSON(mustMarshalConfig(cfg))
		if err != nil {
			httpx.WriteError(w, http.StatusBadRequest, err.Error())
			return
		}
		p.ConfigJSON = normalized
		if err := h.projects.Update(&p); err != nil {
			writeRepoError(w, err)
			return
		}
		_ = h.runner.Scheduler().ReloadProject(p.RecID)
		updated++
	}
	h.audit.Record(r.Context(), r, audit.Entry{
		Action: audit.ActionUpdate, Module: audit.ModuleProject, TargetType: "workspace",
		TargetID: ws.RecID, TargetName: ws.WorkspaceName,
		Summary: "批量配置构建后邮件通知",
		Detail:  "projects_updated=" + strconv.Itoa(updated),
		Success: true,
	})
	httpx.WriteJSON(w, http.StatusOK, batchPostBuildEmailResponse{Updated: updated})
}

func mustMarshalConfig(cfg projectcfg.Config) string {
	b, err := json.Marshal(cfg)
	if err != nil {
		return "{}"
	}
	return string(b)
}
