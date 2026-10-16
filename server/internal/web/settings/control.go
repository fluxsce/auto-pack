package settings

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"auto-pack/server/internal/access"
	"auto-pack/server/internal/audit"
	"auto-pack/server/internal/iam"
	"auto-pack/server/internal/serverupdate"
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

// UpgradeStatus 返回当前版本与是否可在本机界面升级。
func (c *Controller) UpgradeStatus(w http.ResponseWriter, r *http.Request) {
	actor := middleware.ActorFrom(r)
	if err := c.requireSettingsWrite(r, actor); err != nil {
		httpx.WriteSvcErr(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, serverupdate.Probe())
}

// UpgradeServer 接收发行包并安排控制台重启（owner / settings.write）。
func (c *Controller) UpgradeServer(w http.ResponseWriter, r *http.Request) {
	actor := middleware.ActorFrom(r)
	if err := c.requireSettingsWrite(r, actor); err != nil {
		httpx.WriteSvcErr(w, err)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, serverupdate.PackMaxBytes+1)
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) || strings.Contains(err.Error(), "request body too large") {
			httpx.WriteError(w, http.StatusBadRequest, "发行包超过 512MB")
			return
		}
		httpx.WriteError(w, http.StatusBadRequest, "请选择发行包")
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "请选择发行包")
		return
	}
	defer file.Close()
	tmp, err := os.CreateTemp("", "ap-server-pack-*")
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "无法保存发行包")
		return
	}
	defer os.Remove(tmp.Name())
	written, copyErr := io.Copy(tmp, io.LimitReader(file, serverupdate.PackMaxBytes+1))
	closeErr := tmp.Close()
	if copyErr != nil {
		httpx.WriteError(w, http.StatusBadRequest, "读取发行包失败")
		return
	}
	if closeErr != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "无法保存发行包")
		return
	}
	if written > serverupdate.PackMaxBytes {
		httpx.WriteError(w, http.StatusBadRequest, "发行包超过 512MB")
		return
	}
	name := filepath.Base(header.Filename)
	sum := strings.TrimSpace(r.FormValue("sha256"))
	msg, err := serverupdate.Apply(tmp.Name(), name, sum)
	c.audit.Note(r.Context(), actor, access.DefaultWorkspaceRecID, audit.ActionServerUpgrade, "server", name, "upgrade "+name, err)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"message": msg})
}

func (c *Controller) requireSettingsWrite(r *http.Request, actor iam.Actor) error {
	if c.settings == nil {
		return iam.ErrForbidden
	}
	return c.settings.AuthorizeWrite(r.Context(), actor, access.DefaultWorkspaceRecID)
}
