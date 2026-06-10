package app

import (
	"database/sql"
	"net/http"

	"auto-pack/backend/internal/httpx"
	"auto-pack/backend/internal/web"
	"auto-pack/backend/pkg/boottime"
	"auto-pack/backend/pkg/config"
)

// NewHandler wires dependencies and returns the root HTTP handler.
func NewHandler(db *sql.DB, cfg config.Config) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc(web.APIPath("health"), func(w http.ResponseWriter, _ *http.Request) {
		httpx.WriteJSON(w, http.StatusOK, map[string]string{
			"status":     "ok",
			"version":    config.AppVersion,
			"started_at": boottime.StartedAtRFC3339Nano(),
		})
	})

	web.Register(mux, web.NewDeps(db, cfg.JWTSecret, cfg.FrontendPath, cfg.DataDir()))

	return httpx.WithCORS(mux)
}
