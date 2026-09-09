package users

import (
	"encoding/json"
	"net/http"

	"auto-pack/server/internal/audit"
	"auto-pack/server/internal/user"
	"auto-pack/server/internal/web/httpx"
	"auto-pack/server/internal/web/middleware"
)

// Controller 处理账号与角色动作矩阵 HTTP。
type Controller struct {
	users *user.Service
	audit *audit.Service
}

// New 构造用户控制器。
func New(users *user.Service, audit *audit.Service) *Controller {
	return &Controller{users: users, audit: audit}
}

// List 列出空间内用户。
func (c *Controller) List(w http.ResponseWriter, r *http.Request) {
	list, err := c.users.List(r.Context(), middleware.ActorFrom(r), r.URL.Query().Get("workspace_rec_id"))
	if httpx.WriteSvcErr(w, err) {
		return
	}
	httpx.WriteJSON(w, http.StatusOK, list)
}

// Create 新建账号并加入空间。
func (c *Controller) Create(w http.ResponseWriter, r *http.Request) {
	var body struct {
		WorkspaceID string `json:"workspace_rec_id"`
		LoginName   string `json:"login_name"`
		DisplayName string `json:"display_name"`
		Password    string `json:"password"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	actor := middleware.ActorFrom(r)
	u, err := c.users.Create(r.Context(), actor, body.WorkspaceID, body.LoginName, body.DisplayName, body.Password, true)
	c.audit.Note(r.Context(), actor, body.WorkspaceID, audit.ActionUserCreate, "user", u.RecID, "create "+body.LoginName, err)
	if httpx.WriteSvcErr(w, err) {
		return
	}
	httpx.WriteJSON(w, http.StatusOK, u)
}

// ListRoleActions 读取角色允许的动作。
func (c *Controller) ListRoleActions(w http.ResponseWriter, r *http.Request) {
	list, err := c.users.ListRoleActions(r.Context(), middleware.ActorFrom(r), r.URL.Query().Get("workspace_rec_id"), r.PathValue("role"))
	if httpx.WriteSvcErr(w, err) {
		return
	}
	httpx.WriteJSON(w, http.StatusOK, list)
}

// ReplaceRoleActions 覆盖角色动作矩阵。
func (c *Controller) ReplaceRoleActions(w http.ResponseWriter, r *http.Request) {
	var body struct {
		WorkspaceID string   `json:"workspace_rec_id"`
		Actions     []string `json:"actions"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	actor := middleware.ActorFrom(r)
	role := r.PathValue("role")
	err := c.users.ReplaceRoleActions(r.Context(), actor, body.WorkspaceID, role, body.Actions)
	c.audit.Note(r.Context(), actor, body.WorkspaceID, audit.ActionRoleReplace, "role", role, "replace actions for "+role, err)
	if err != nil {
		httpx.WriteSvcErr(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
