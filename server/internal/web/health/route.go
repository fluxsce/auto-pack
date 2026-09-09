package health

import (
	"auto-pack/server/internal/web/mod"
)

const moduleName = "health"

func init() {
	mod.Register(moduleName, Init)
}

// Init 挂 GET /autopack/api/health，不鉴权。
func Init(d mod.Deps) {
	c := New(d.StartedAt)
	d.Mux.HandleFunc("GET "+mod.Path("health"), c.Get)
}
