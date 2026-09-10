package build

import (
	"net/http"

	"auto-pack/server/internal/web/mod"
)

const moduleName = "build"

func init() {
	mod.Register(moduleName, Init)
}

// Init 挂 /autopack/api/build：工作空间自动构建。
func Init(d mod.Deps) {
	c := New(d.Build, d.Audit)
	d.Mux.Handle("POST "+mod.Path("build"), d.Authz(http.HandlerFunc(c.Start)))
	d.Mux.Handle("GET "+mod.Path("build", "status"), d.Authz(http.HandlerFunc(c.Status)))
	d.Mux.Handle("GET "+mod.Path("build", "history"), d.Authz(http.HandlerFunc(c.History)))
	d.Mux.Handle("GET "+mod.Path("build", "history", "{buildID}", "log"), d.Authz(http.HandlerFunc(c.Log)))
}
