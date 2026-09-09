package users

import (
	"net/http"

	"auto-pack/server/internal/web/mod"
)

const moduleName = "users"

func init() {
	mod.Register(moduleName, Init)
}

// Init 挂 /autopack/api/users 与 /roles/{role}/actions。
func Init(d mod.Deps) {
	c := New(d.Users, d.Audit)
	d.Mux.Handle("GET "+mod.Path("users"), d.Authz(http.HandlerFunc(c.List)))
	d.Mux.Handle("POST "+mod.Path("users"), d.Authz(http.HandlerFunc(c.Create)))
	d.Mux.Handle("GET "+mod.Path("roles", "{role}", "actions"), d.Authz(http.HandlerFunc(c.ListRoleActions)))
	d.Mux.Handle("PUT "+mod.Path("roles", "{role}", "actions"), d.Authz(http.HandlerFunc(c.ReplaceRoleActions)))
}
