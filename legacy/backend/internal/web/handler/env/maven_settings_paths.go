package env

import (
	"net/http"
	"strings"

	"auto-pack/backend/internal/httpx"
	"auto-pack/backend/pkg/maven"
)

func (h *Handler) mavenSettingsPaths(w http.ResponseWriter, r *http.Request) {
	mavenHome := strings.TrimSpace(r.URL.Query().Get("maven_home"))
	items := maven.SettingsCandidates(mavenHome)
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": items})
}
