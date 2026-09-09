package jobs

import (
	"net/http"

	"auto-pack/server/internal/web/mod"
)

const moduleName = "jobs"

func init() {
	mod.Register(moduleName, Init)
}

// Init 挂 /autopack/api/jobs。
func Init(d mod.Deps) {
	c := New(d.Jobs, d.Projects, d.Audit)
	d.Mux.Handle("GET "+mod.Path("jobs"), d.Authz(http.HandlerFunc(c.List)))
	d.Mux.Handle("POST "+mod.Path("jobs"), d.Authz(http.HandlerFunc(c.Submit)))
	d.Mux.Handle("POST "+mod.Path("jobs", "batch"), d.Authz(http.HandlerFunc(c.SubmitBatch)))
	d.Mux.Handle("GET "+mod.Path("jobs", "{id}"), d.Authz(http.HandlerFunc(c.Get)))
	d.Mux.Handle("GET "+mod.Path("jobs", "{id}", "log"), d.Authz(http.HandlerFunc(c.Log)))
	d.Mux.Handle("POST "+mod.Path("jobs", "{id}", "cancel"), d.Authz(http.HandlerFunc(c.Cancel)))
}
