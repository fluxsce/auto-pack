package envs

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"auto-pack/server/internal/audit"
	"auto-pack/server/internal/env"
	"auto-pack/server/internal/web/httpx"
	"auto-pack/server/internal/web/middleware"
)

// Controller 处理环境凭据 HTTP，只回 hint 与公开字段，不回明文密钥。
type Controller struct {
	envs  *env.Service
	audit *audit.Service
}

// New 构造环境控制器。
func New(envs *env.Service, audit *audit.Service) *Controller {
	return &Controller{envs: envs, audit: audit}
}

// Kinds 返回支持的环境类型与字段目录。
func (c *Controller) Kinds(w http.ResponseWriter, r *http.Request) {
	_ = r
	if c.envs == nil {
		httpx.WriteJSON(w, http.StatusOK, []env.KindSpec{})
		return
	}
	httpx.WriteJSON(w, http.StatusOK, c.envs.ListKinds())
}

// List 列出空间内环境。
func (c *Controller) List(w http.ResponseWriter, r *http.Request) {
	list, err := c.envs.List(r.Context(), middleware.ActorFrom(r), r.URL.Query().Get("workspace_rec_id"))
	if httpx.WriteSvcErr(w, err) {
		return
	}
	httpx.WriteJSON(w, http.StatusOK, list)
}

// Get 读取单条环境（密钥字段留空，secrets_set 标明已保存）。
func (c *Controller) Get(w http.ResponseWriter, r *http.Request) {
	out, err := c.envs.Get(r.Context(), middleware.ActorFrom(r), r.PathValue("id"))
	if httpx.WriteSvcErr(w, err) {
		return
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}

// Save 新建或更新环境。
func (c *Controller) Save(w http.ResponseWriter, r *http.Request) {
	var e env.Env
	if err := json.NewDecoder(r.Body).Decode(&e); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid body")
		return
	}
	actor := middleware.ActorFrom(r)
	out, err := c.envs.Save(r.Context(), actor, e)
	c.audit.Note(r.Context(), actor, e.WorkspaceRecID, audit.ActionEnvSave, "env", firstNonEmpty(out.RecID, e.RecID), "save "+e.EnvName, err)
	if httpx.WriteSvcErr(w, err) {
		return
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}

// Probe 探测草稿或已存环境，失败也回 200 + ok=false。
func (c *Controller) Probe(w http.ResponseWriter, r *http.Request) {
	if c.envs == nil {
		httpx.WriteError(w, http.StatusBadRequest, "env service not configured")
		return
	}
	var in env.ProbeIn
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid body")
		return
	}
	out, err := c.envs.Probe(r.Context(), middleware.ActorFrom(r), in)
	if httpx.WriteSvcErr(w, err) {
		return
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}

// SendMail 用已存 SMTP 环境发送一封告知邮件。
func (c *Controller) SendMail(w http.ResponseWriter, r *http.Request) {
	var in env.MailIn
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid body")
		return
	}
	actor := middleware.ActorFrom(r)
	id := r.PathValue("id")
	row, _ := c.envs.Get(r.Context(), actor, id)
	err := c.envs.SendMail(r.Context(), actor, id, in)
	c.audit.Note(r.Context(), actor, row.WorkspaceRecID, audit.ActionEnvMail, "env", id, "mail "+row.EnvName+" → "+strings.TrimSpace(in.Recipients), err)
	if err == nil {
		httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
		return
	}
	if httpx.WriteIAM(w, err) {
		return
	}
	if errors.Is(err, env.ErrMailSMTP) || errors.Is(err, env.ErrMailDisabled) || errors.Is(err, env.ErrMailRecipients) || errors.Is(err, env.ErrMailSubject) || errors.Is(err, env.ErrMailBody) || errors.Is(err, env.ErrMailTemplate) {
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	if strings.Contains(err.Error(), "not found") || strings.Contains(strings.ToLower(err.Error()), "sql: no rows") {
		httpx.WriteError(w, http.StatusNotFound, err.Error())
		return
	}
	httpx.WriteError(w, http.StatusBadGateway, err.Error())
}

// Delete 删除环境。
func (c *Controller) Delete(w http.ResponseWriter, r *http.Request) {
	actor := middleware.ActorFrom(r)
	id := r.PathValue("id")
	row, _ := c.envs.Get(r.Context(), actor, id)
	err := c.envs.Delete(r.Context(), actor, id)
	c.audit.Note(r.Context(), actor, row.WorkspaceRecID, audit.ActionEnvDelete, "env", id, "delete "+row.EnvName, err)
	if httpx.WriteSvcErr(w, err) {
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
