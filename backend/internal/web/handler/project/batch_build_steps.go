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

type batchBuildStepsPayload struct {
	WorkspaceRecID        string `json:"workspace_rec_id"`
	ProjectType           string `json:"project_type"`
	UpdateBuildCmd        bool   `json:"update_build_cmd"`
	BuildCmd              string `json:"build_cmd"`
	UpdateOutputDir       bool   `json:"update_output_dir"`
	OutputDir             string `json:"output_dir"`
	UpdateArtifactPattern bool   `json:"update_artifact_pattern"`
	ArtifactPattern       string `json:"artifact_pattern"`
	UpdateLogRetention    bool   `json:"update_log_retention"`
	LogRetentionDays      int    `json:"log_retention_days"`
}

type batchBuildStepsResponse struct {
	Updated int `json:"updated"`
}

func (h *Handler) batchBuildSteps(w http.ResponseWriter, r *http.Request) {
	var body batchBuildStepsPayload
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	wsID := strings.TrimSpace(body.WorkspaceRecID)
	if wsID == "" {
		httpx.WriteError(w, http.StatusBadRequest, "workspace_rec_id is required")
		return
	}
	projectType := strings.TrimSpace(strings.ToLower(body.ProjectType))
	if projectType != "" && !projectcfg.IsValidType(projectType) {
		httpx.WriteError(w, http.StatusBadRequest, "invalid project_type")
		return
	}
	opts := projectcfg.BuildStepsBulkOpts{
		UpdateBuildCmd:        body.UpdateBuildCmd,
		BuildCmd:              body.BuildCmd,
		UpdateOutputDir:       body.UpdateOutputDir,
		OutputDir:             body.OutputDir,
		UpdateArtifactPattern: body.UpdateArtifactPattern,
		ArtifactPattern:       body.ArtifactPattern,
		UpdateLogRetention:    body.UpdateLogRetention,
		LogRetentionDays:      body.LogRetentionDays,
	}
	if err := projectcfg.ValidateBuildStepsBulk(opts); err != nil {
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
	projects, err := h.projects.List(wsID, projectType)
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
		projectcfg.ApplyBuildStepsToConfig(&cfg, opts)
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
		Summary: "批量调整构建步骤",
		Detail:  "projects_updated=" + strconv.Itoa(updated),
		Success: true,
	})
	httpx.WriteJSON(w, http.StatusOK, batchBuildStepsResponse{Updated: updated})
}
