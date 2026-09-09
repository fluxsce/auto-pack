package projects

import (
	"net/http"

	"auto-pack/server/internal/web/mod"
)

const moduleName = "projects"

func init() {
	mod.Register(moduleName, Init)
}

// Init 挂 /autopack/api/projects。
func Init(d mod.Deps) {
	c := New(d.Projects, d.Workspaces, d.Envs, d.Settings, d.Audit)
	d.Mux.Handle("GET "+mod.Path("projects"), d.Authz(http.HandlerFunc(c.List)))
	d.Mux.Handle("POST "+mod.Path("projects"), d.Authz(http.HandlerFunc(c.Save)))
	d.Mux.Handle("GET "+mod.Path("projects", "cron-preview"), d.Authz(http.HandlerFunc(c.PreviewCron)))
	d.Mux.Handle("POST "+mod.Path("projects", "cron-preview"), d.Authz(http.HandlerFunc(c.PreviewCron)))
	d.Mux.Handle("POST "+mod.Path("projects", "git-branches"), d.Authz(http.HandlerFunc(c.UpdateGitBranches)))
	d.Mux.Handle("POST "+mod.Path("projects", "post-actions"), d.Authz(http.HandlerFunc(c.UpdatePostActions)))
	d.Mux.Handle("GET "+mod.Path("projects", "{id}"), d.Authz(http.HandlerFunc(c.Get)))
	d.Mux.Handle("GET "+mod.Path("projects", "{id}", "files"), d.Authz(http.HandlerFunc(c.Files)))
	d.Mux.Handle("POST "+mod.Path("projects", "{id}", "files", "clear"), d.Authz(http.HandlerFunc(c.ClearFiles)))
	d.Mux.Handle("GET "+mod.Path("projects", "{id}", "file"), d.Authz(http.HandlerFunc(c.File)))
	d.Mux.Handle("GET "+mod.Path("projects", "{id}", "file", "download"), d.Authz(http.HandlerFunc(c.DownloadFile)))
	d.Mux.Handle("DELETE "+mod.Path("projects", "{id}", "file"), d.Authz(http.HandlerFunc(c.DeleteFile)))
	d.Mux.Handle("GET "+mod.Path("projects", "{id}", "deps"), d.Authz(http.HandlerFunc(c.Deps)))
	d.Mux.Handle("POST "+mod.Path("projects", "{id}", "remote-note"), d.Authz(http.HandlerFunc(c.RemoteNote)))
	d.Mux.Handle("POST "+mod.Path("projects", "{id}", "clone"), d.Authz(http.HandlerFunc(c.Clone)))
	d.Mux.Handle("DELETE "+mod.Path("projects", "{id}"), d.Authz(http.HandlerFunc(c.Delete)))
}
