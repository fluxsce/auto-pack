package env

import (
	"net/http"

	"auto-pack/backend/internal/httpx"
	"auto-pack/backend/pkg/nodejs"
)

func (h *Handler) npmrcPaths(w http.ResponseWriter, _ *http.Request) {
	items := nodejs.NpmrcCandidates()
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": items})
}
