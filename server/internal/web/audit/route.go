package audit

import (
	"net/http"

	"auto-pack/server/internal/web/mod"
)

const moduleName = "audit"

func init() {
	mod.Register(moduleName, Init)
}

// Init 挂 GET /autopack/api/audit。
func Init(d mod.Deps) {
	c := New(d.Audit)
	d.Mux.Handle("GET "+mod.Path("audit"), d.Authz(http.HandlerFunc(c.List)))
}
