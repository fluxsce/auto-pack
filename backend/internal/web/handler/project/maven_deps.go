package project

import (
	"net/http"
	"strings"

	"auto-pack/backend/internal/httpx"
	"auto-pack/backend/pkg/mavendeps"
)

// mavenDependencyAnalysis handles GET /projects/{recID}/maven-dependency-analysis
func (h *Handler) mavenDependencyAnalysis(w http.ResponseWriter, r *http.Request) {
	recID := strings.TrimSpace(r.PathValue("recID"))
	if recID == "" {
		httpx.WriteError(w, http.StatusBadRequest, "rec_id required")
		return
	}
	project, err := h.projects.FindByRecID(recID)
	if err != nil {
		httpx.WriteError(w, http.StatusNotFound, "project not found")
		return
	}
	settingsXML := strings.TrimSpace(r.URL.Query().Get("settings_xml"))
	analysis, modules, err := h.runner.AnalyzeMavenWorkspace(project.WorkspaceRecID, recID, settingsXML)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"analysis":         analysis,
		"modules":          modules,
		"install_command":  mavendeps.DefaultInstallCmd,
		"workspace_rec_id": project.WorkspaceRecID,
	})
}
