package app

import (
	"database/sql"
	"net/http"

	"auto-pack/backend/internal/httpx"
	"auto-pack/backend/internal/web"
	"auto-pack/backend/pkg/boottime"
	"auto-pack/backend/pkg/config"
	"auto-pack/backend/pkg/git"
	"auto-pack/backend/pkg/logger"
	"auto-pack/backend/pkg/packrun"
)

// NewHandler wires dependencies and returns the root HTTP handler.
func NewHandler(db *sql.DB, cfg config.Config) http.Handler {
	// Process-wide shallow depth for pack sync / release clones (app.yaml git.clone_depth).
	git.SetGlobalCloneDepth(cfg.GitCloneDepth)
	logger.Info("git clone depth configured", "clone_depth", git.GlobalCloneDepth())

	// Per-workspace pre-sync with a global concurrency cap (app.yaml git.sync_max_workspaces).
	packrun.SetWorkspaceSyncMaxConcurrent(cfg.GitSyncMaxWorkspaces)
	logger.Info("workspace source sync concurrency configured",
		"sync_max_workspaces", packrun.WorkspaceSyncMaxConcurrent())

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
