// Package router 组装 CORS、静态页与已自注册的功能模块。本包不写业务路径。
package router

import (
	"net/http"

	"auto-pack/server/internal/token"
	"auto-pack/server/internal/web/httpx"
	"auto-pack/server/internal/web/middleware"
	"auto-pack/server/internal/web/mod"
	"auto-pack/server/internal/web/static"
)

// Handler 返回根处理器：模块 API + 可选静态控制台。
func Handler(d mod.Deps) http.Handler {
	mux := http.NewServeMux()
	d.Mux = mux
	tokens, _ := d.Tokens.(*token.Issuer)
	d.Authz = middleware.RequireAuth(tokens)
	mod.Apply(d)
	if root, err := static.ResolveRoot(d.Frontend); err == nil {
		_ = static.Mount(mux, mod.PageRoot, root)
	}
	return httpx.WithCORS(httpx.WithRecover(mux))
}
