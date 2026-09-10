package router

import (
	_ "auto-pack/server/internal/web/audit"
	_ "auto-pack/server/internal/web/auth/route"
	_ "auto-pack/server/internal/web/build"
	_ "auto-pack/server/internal/web/envs"
	_ "auto-pack/server/internal/web/health"
	_ "auto-pack/server/internal/web/jobs"
	_ "auto-pack/server/internal/web/projects"
	_ "auto-pack/server/internal/web/release"
	_ "auto-pack/server/internal/web/settings"
	_ "auto-pack/server/internal/web/users"
	_ "auto-pack/server/internal/web/workspaces"
)
