package envs

import (
	"net/http"

	"auto-pack/server/internal/web/mod"
)

const moduleName = "envs"

func init() {
	mod.Register(moduleName, Init)
}

// Init 挂 /autopack/api/envs。
func Init(d mod.Deps) {
	c := New(d.Envs, d.Audit)
	d.Mux.Handle("GET "+mod.Path("envs"), d.Authz(http.HandlerFunc(c.List)))
	d.Mux.Handle("GET "+mod.Path("envs", "kinds"), d.Authz(http.HandlerFunc(c.Kinds)))
	d.Mux.Handle("POST "+mod.Path("envs", "probe"), d.Authz(http.HandlerFunc(c.Probe)))
	d.Mux.Handle("GET "+mod.Path("envs", "{id}"), d.Authz(http.HandlerFunc(c.Get)))
	d.Mux.Handle("POST "+mod.Path("envs", "{id}", "mail"), d.Authz(http.HandlerFunc(c.SendMail)))
	d.Mux.Handle("POST "+mod.Path("envs"), d.Authz(http.HandlerFunc(c.Save)))
	d.Mux.Handle("DELETE "+mod.Path("envs", "{id}"), d.Authz(http.HandlerFunc(c.Delete)))
}
