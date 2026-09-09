package workspaces

import (
	"net/http"

	"auto-pack/server/internal/web/mod"
)

const moduleName = "workspaces"

func init() {
	mod.Register(moduleName, Init)
}

// Init 挂 /autopack/api/workspaces。
func Init(d mod.Deps) {
	c := New(d.Workspaces, d.Audit)
	d.Mux.Handle("GET "+mod.Path("workspaces"), d.Authz(http.HandlerFunc(c.List)))
	d.Mux.Handle("POST "+mod.Path("workspaces"), d.Authz(http.HandlerFunc(c.Create)))
	d.Mux.Handle("POST "+mod.Path("workspaces", "clone"), d.Authz(http.HandlerFunc(c.Clone)))
	d.Mux.Handle("POST "+mod.Path("workspaces", "import"), d.Authz(http.HandlerFunc(c.Import)))
	d.Mux.Handle("GET "+mod.Path("workspaces", "{id}", "export"), d.Authz(http.HandlerFunc(c.Export)))
	d.Mux.Handle("PUT "+mod.Path("workspaces", "{id}"), d.Authz(http.HandlerFunc(c.Update)))
	d.Mux.Handle("DELETE "+mod.Path("workspaces", "{id}"), d.Authz(http.HandlerFunc(c.Delete)))
	d.Mux.Handle("GET "+mod.Path("workspaces", "{id}", "members"), d.Authz(http.HandlerFunc(c.ListMembers)))
	d.Mux.Handle("POST "+mod.Path("workspaces", "{id}", "members"), d.Authz(http.HandlerFunc(c.AddMember)))
	d.Mux.Handle("DELETE "+mod.Path("workspaces", "{id}", "members", "{userId}"), d.Authz(http.HandlerFunc(c.RemoveMember)))
}
