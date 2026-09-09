package release

import (
	"net/http"

	"auto-pack/server/internal/web/mod"
)

const moduleName = "release"

func init() {
	mod.Register(moduleName, Init)
}

// Init 挂 /autopack/api/release：自动发版、源码预同步、Maven 构建顺序。
func Init(d mod.Deps) {
	c := New(d.Release, d.Audit)
	d.Mux.Handle("POST "+mod.Path("release"), d.Authz(http.HandlerFunc(c.Start)))
	d.Mux.Handle("GET "+mod.Path("release", "defaults"), d.Authz(http.HandlerFunc(c.Defaults)))
	d.Mux.Handle("GET "+mod.Path("release", "status"), d.Authz(http.HandlerFunc(c.Status)))
	d.Mux.Handle("PUT "+mod.Path("release", "post-options"), d.Authz(http.HandlerFunc(c.SavePostOptions)))
	d.Mux.Handle("GET "+mod.Path("release", "history"), d.Authz(http.HandlerFunc(c.History)))
	d.Mux.Handle("GET "+mod.Path("release", "history", "{releaseID}", "log"), d.Authz(http.HandlerFunc(c.Log)))
	d.Mux.Handle("POST "+mod.Path("release", "sync-sources"), d.Authz(http.HandlerFunc(c.SyncSources)))
	d.Mux.Handle("GET "+mod.Path("release", "maven-order"), d.Authz(http.HandlerFunc(c.MavenOrder)))
}
