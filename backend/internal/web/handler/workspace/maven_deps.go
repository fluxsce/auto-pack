package workspace

import (
	"net/http"
	"strings"

	"auto-pack/backend/internal/httpx"
	"auto-pack/backend/pkg/mavendeps"
)

type mavenWorkspaceAnalyzer interface {
	AnalyzeMavenWorkspaceAll(workspaceRecID, settingsXML string) (*mavendeps.WorkspaceAnalysisResult, []mavendeps.Module, error)
}

// mavenDependencyAnalysis handles GET /workspaces/{recID}/maven-dependency-analysis
func (h *Handler) mavenDependencyAnalysis(w http.ResponseWriter, r *http.Request) {
	wsRecID := strings.TrimSpace(r.PathValue("recID"))
	if wsRecID == "" {
		httpx.WriteError(w, http.StatusBadRequest, "workspace rec_id required")
		return
	}
	if _, err := h.repo.FindByRecID(wsRecID); err != nil {
		httpx.WriteError(w, http.StatusNotFound, "workspace not found")
		return
	}
	analyzer, ok := h.releaseGuard.(mavenWorkspaceAnalyzer)
	if !ok {
		httpx.WriteError(w, http.StatusInternalServerError, "build runner not available")
		return
	}
	settingsXML := strings.TrimSpace(r.URL.Query().Get("settings_xml"))
	analysis, modules, err := analyzer.AnalyzeMavenWorkspaceAll(wsRecID, settingsXML)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"analysis":         analysis,
		"modules":          modules,
		"install_command":  mavendeps.DefaultInstallCmd,
		"workspace_rec_id": wsRecID,
	})
}
