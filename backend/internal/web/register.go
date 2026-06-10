package web

import (
	"net/http"

	"auto-pack/backend/internal/repository"
	"auto-pack/backend/internal/web/auth"
	envhandler "auto-pack/backend/internal/web/handler/env"
	"auto-pack/backend/internal/web/handler/login"
	"auto-pack/backend/internal/web/handler/me"
	operationloghandler "auto-pack/backend/internal/web/handler/operationlog"
	"auto-pack/backend/internal/web/handler/password"
	projecthandler "auto-pack/backend/internal/web/handler/project"
	userhandler "auto-pack/backend/internal/web/handler/user"
	"auto-pack/backend/internal/web/handler/username"
	workspacehandler "auto-pack/backend/internal/web/handler/workspace"
	"auto-pack/backend/internal/web/middleware"
	"auto-pack/backend/internal/web/static"
	"auto-pack/backend/pkg/audit"
	"auto-pack/backend/pkg/logger"
)

// Deps holds web-layer HTTP dependencies.
type Deps struct {
	Auth                 *auth.Service
	Tokens               *auth.TokenIssuer
	UserRepo             *repository.UserRepository
	OpLogRepo            *repository.OperationLogRepository
	EnvRepo              *repository.EnvRepository
	ProjectRepo          *repository.ProjectRepository
	BuildRepo            *repository.BuildRepository
	WorkspaceReleaseRepo *repository.WorkspaceReleaseRepository
	WorkspaceRepo        *repository.WorkspaceRepository
	FrontendPath         string
	DataDir              string
}

// Register mounts auth API and on-disk SPA routes (gateway-style, under BasePath).
func Register(mux *http.ServeMux, deps Deps) {
	requireAuth := middleware.RequireAuth(deps.Tokens)

	auditRec := audit.NewRecorder(deps.OpLogRepo, deps.UserRepo)

	mux.Handle(APIPath("auth", "login"), login.NewHandler(deps.Auth, auditRec))
	mux.Handle(APIPath("auth", "me"), requireAuth(me.NewHandler(deps.Auth)))
	mux.Handle(APIPath("auth", "password"), requireAuth(password.NewHandler(deps.Auth)))
	mux.Handle(APIPath("auth", "username"), requireAuth(username.NewHandler(deps.Auth)))

	userhandler.Register(mux, userhandler.NewHandler(deps.UserRepo, auditRec), requireAuth, APIPath())
	operationloghandler.Register(mux, operationloghandler.NewHandler(deps.OpLogRepo), requireAuth, APIPath())

	envhandler.Register(mux, envhandler.NewHandler(deps.EnvRepo, auditRec), requireAuth, APIPath())
	projectH := projecthandler.NewHandler(deps.ProjectRepo, deps.WorkspaceRepo, deps.EnvRepo, deps.BuildRepo, deps.WorkspaceReleaseRepo, deps.DataDir, auditRec)
	workspacehandler.Register(mux, workspacehandler.NewHandler(deps.WorkspaceRepo, deps.ProjectRepo, deps.BuildRepo, deps.WorkspaceReleaseRepo, projectH, projectH, deps.DataDir, auditRec), requireAuth, APIPath())
	projecthandler.Register(mux, projectH, requireAuth, APIPath())

	root, err := static.ResolveRoot(deps.FrontendPath)
	if err != nil {
		logger.Warn("frontend static disabled", "err", err)
		return
	}
	if err := static.Mount(mux, BasePath, root); err != nil {
		logger.Warn("frontend static mount failed", "err", err)
	}
}
