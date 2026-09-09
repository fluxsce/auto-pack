// Package route 挂登录路径。与 token 包拆开，避免 web/mod 与 auth 环依赖。
package route

import (
	"net/http"

	"auto-pack/server/internal/token"
	"auto-pack/server/internal/web/auth/control"
	"auto-pack/server/internal/web/mod"
)

const moduleName = "auth"

func init() {
	mod.Register(moduleName, Init)
}

// Init 挂 /autopack/api/auth/login 与 /auth/me。
func Init(d mod.Deps) {
	tokens, ok := d.Tokens.(*token.Issuer)
	if !ok || tokens == nil {
		return
	}
	c := control.New(d.Users, tokens)
	d.Mux.HandleFunc("POST "+mod.Path("auth", "login"), c.Login)
	d.Mux.Handle("GET "+mod.Path("auth", "me"), d.Authz(http.HandlerFunc(c.Me)))
}
