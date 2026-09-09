package settings

import (
	"net/http"

	"auto-pack/server/internal/web/mod"
)

const moduleName = "settings"

func init() {
	mod.Register(moduleName, Init)
}

// Init 挂 /autopack/api/settings。
func Init(d mod.Deps) {
	c := New(d.Settings, d.Audit)
	d.Mux.Handle("GET "+mod.Path("settings", "props"), d.Authz(http.HandlerFunc(c.GetProps)))
	d.Mux.Handle("PUT "+mod.Path("settings", "props"), d.Authz(http.HandlerFunc(c.SaveProps)))
	d.Mux.Handle("GET "+mod.Path("settings", "vars"), d.Authz(http.HandlerFunc(c.ListVars)))
	d.Mux.Handle("POST "+mod.Path("settings", "vars"), d.Authz(http.HandlerFunc(c.SaveVar)))
	d.Mux.Handle("DELETE "+mod.Path("settings", "vars", "{key}"), d.Authz(http.HandlerFunc(c.DeleteVar)))
}
