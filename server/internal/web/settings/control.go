package settings

import (
	"encoding/json"
	"net/http"
	"net/url"

	"auto-pack/server/internal/audit"
	appsettings "auto-pack/server/internal/settings"
	"auto-pack/server/internal/web/httpx"
	"auto-pack/server/internal/web/middleware"
)

// Controller 处理系统属性与全局变量 HTTP。
type Controller struct {
	settings *appsettings.Service
	audit    *audit.Service
}

// New 构造设置控制器。
func New(settings *appsettings.Service, audit *audit.Service) *Controller {
	return &Controller{settings: settings, audit: audit}
}

// GetProps 读取系统属性。
func (c *Controller) GetProps(w http.ResponseWriter, r *http.Request) {
	if c.settings == nil {
		httpx.WriteJSON(w, http.StatusOK, appsettings.DefaultProps())
		return
	}
	out, err := c.settings.GetProps(r.Context(), middleware.ActorFrom(r), r.URL.Query().Get("workspace_rec_id"))
	if httpx.WriteSvcErr(w, err) {
		return
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}

// SaveProps 保存系统属性。
func (c *Controller) SaveProps(w http.ResponseWriter, r *http.Request) {
	if c.settings == nil {
		httpx.WriteError(w, http.StatusBadRequest, "settings service not configured")
		return
	}
	var in appsettings.Props
	_ = json.NewDecoder(r.Body).Decode(&in)
	actor := middleware.ActorFrom(r)
	ws := r.URL.Query().Get("workspace_rec_id")
	out, err := c.settings.SaveProps(r.Context(), actor, ws, in)
	c.audit.Note(r.Context(), actor, ws, audit.ActionSettingsSave, "settings", "props", "save system props", err)
	if httpx.WriteSvcErr(w, err) {
		return
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}

// ListVars 列出全局变量。
func (c *Controller) ListVars(w http.ResponseWriter, r *http.Request) {
	if c.settings == nil {
		httpx.WriteJSON(w, http.StatusOK, []appsettings.Var{})
		return
	}
	list, err := c.settings.ListVars(r.Context(), middleware.ActorFrom(r), r.URL.Query().Get("workspace_rec_id"))
	if httpx.WriteSvcErr(w, err) {
		return
	}
	httpx.WriteJSON(w, http.StatusOK, list)
}

// SaveVar 写入一条全局变量。
func (c *Controller) SaveVar(w http.ResponseWriter, r *http.Request) {
	if c.settings == nil {
		httpx.WriteError(w, http.StatusBadRequest, "settings service not configured")
		return
	}
	var in appsettings.Var
	_ = json.NewDecoder(r.Body).Decode(&in)
	actor := middleware.ActorFrom(r)
	ws := r.URL.Query().Get("workspace_rec_id")
	out, err := c.settings.SaveVar(r.Context(), actor, ws, in)
	c.audit.Note(r.Context(), actor, ws, audit.ActionVarSave, "var", out.RecID, "save "+in.Key, err)
	if httpx.WriteSvcErr(w, err) {
		return
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}

// DeleteVar 删除一条全局变量。
func (c *Controller) DeleteVar(w http.ResponseWriter, r *http.Request) {
	if c.settings == nil {
		httpx.WriteError(w, http.StatusBadRequest, "settings service not configured")
		return
	}
	key, _ := url.PathUnescape(r.PathValue("key"))
	actor := middleware.ActorFrom(r)
	ws := r.URL.Query().Get("workspace_rec_id")
	err := c.settings.DeleteVar(r.Context(), actor, ws, key)
	c.audit.Note(r.Context(), actor, ws, audit.ActionVarDelete, "var", key, "delete "+key, err)
	if httpx.WriteSvcErr(w, err) {
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]bool{"ok": true})
}
