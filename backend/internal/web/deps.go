package web

import (
	"database/sql"

	"auto-pack/backend/internal/repository"
	"auto-pack/backend/internal/web/auth"
)

// NewDeps wires web-layer services from database, JWT secret, and optional frontend dist path.
func NewDeps(db *sql.DB, jwtSecret, frontendPath, dataDir string) Deps {
	userRepo := repository.NewUser(db)
	opLogRepo := repository.NewOperationLog(db)
	envRepo := repository.NewEnv(db)
	projectRepo := repository.NewProject(db)
	buildRepo := repository.NewBuild(db)
	releaseRepo := repository.NewWorkspaceRelease(db)
	pomVersionRepo := repository.NewWorkspacePomVersion(db)
	branchCloneRepo := repository.NewWorkspaceBranchClone(db)
	workspaceRepo := repository.NewWorkspace(db)
	tokens := auth.NewTokenIssuer(jwtSecret)
	return Deps{
		Auth:                     auth.NewService(userRepo, tokens),
		Tokens:                   tokens,
		UserRepo:                 userRepo,
		OpLogRepo:                opLogRepo,
		EnvRepo:                  envRepo,
		ProjectRepo:              projectRepo,
		BuildRepo:                buildRepo,
		WorkspaceReleaseRepo:     releaseRepo,
		WorkspacePomVersionRepo:  pomVersionRepo,
		WorkspaceBranchCloneRepo: branchCloneRepo,
		WorkspaceRepo:            workspaceRepo,
		FrontendPath:             frontendPath,
		DataDir:                  dataDir,
	}
}
