package control

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"auto-pack/server/internal/token"
	"auto-pack/server/internal/user"
	"auto-pack/server/internal/web/httpx"
	"auto-pack/server/internal/web/middleware"
)

// Controller 处理登录与当前用户。
type Controller struct {
	users  *user.Service
	tokens *token.Issuer
}

// New 构造登录控制器。
func New(users *user.Service, tokens *token.Issuer) *Controller {
	return &Controller{users: users, tokens: tokens}
}

// Login 校验账号并发 JWT。
func (c *Controller) Login(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid body")
		return
	}
	u, err := c.users.Authenticate(r.Context(), body.Username, body.Password)
	if err != nil {
		if errors.Is(err, user.ErrAccountDisabled) {
			httpx.WriteError(w, http.StatusForbidden, err.Error())
			return
		}
		httpx.WriteError(w, http.StatusUnauthorized, user.ErrInvalidCredentials.Error())
		return
	}
	tok, exp, err := c.tokens.Issue(u.RecID, u.LoginName)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"token":      tok,
		"expires_at": exp.Format(time.RFC3339),
		"user":       map[string]any{"id": u.RecID, "login_name": u.LoginName, "display_name": u.DisplayName},
	})
}

// Me 返回当前登录用户。
func (c *Controller) Me(w http.ResponseWriter, r *http.Request) {
	actor := middleware.ActorFrom(r)
	u, err := c.users.FindByID(r.Context(), actor.UserID)
	if err != nil {
		httpx.WriteError(w, http.StatusUnauthorized, "user not found")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"id": u.RecID, "login_name": u.LoginName, "display_name": u.DisplayName,
	})
}
