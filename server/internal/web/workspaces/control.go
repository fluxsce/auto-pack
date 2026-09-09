package workspaces

import (
	"encoding/json"
	"net/http"

	"auto-pack/server/internal/audit"
	"auto-pack/server/internal/web/httpx"
	"auto-pack/server/internal/web/middleware"
	"auto-pack/server/internal/workspace"
)

// Controller 处理空间与成员 HTTP。
type Controller struct {
	workspaces *workspace.Service
	audit      *audit.Service
}

// New 构造空间控制器。
func New(workspaces *workspace.Service, audit *audit.Service) *Controller {
	return &Controller{workspaces: workspaces, audit: audit}
}

// List 列出当前用户可见空间。
func (c *Controller) List(w http.ResponseWriter, r *http.Request) {
	list, err := c.workspaces.ListForActor(r.Context(), middleware.ActorFrom(r))
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	httpx.WriteJSON(w, http.StatusOK, list)
}

// Create 新建空间。
func (c *Controller) Create(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Code string `json:"workspace_code"`
		Name string `json:"workspace_name"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	actor := middleware.ActorFrom(r)
	ws, err := c.workspaces.Create(r.Context(), actor, body.Code, body.Name)
	c.audit.Note(r.Context(), actor, ws.RecID, audit.ActionWorkspaceCreate, "workspace", ws.RecID, "create "+body.Name, err)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	httpx.WriteJSON(w, http.StatusOK, ws)
}

// Update 修改空间名称与备注。
func (c *Controller) Update(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name   string `json:"workspace_name"`
		Remark string `json:"remark"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	actor := middleware.ActorFrom(r)
	id := r.PathValue("id")
	ws, err := c.workspaces.Update(r.Context(), actor, id, body.Name, body.Remark)
	c.audit.Note(r.Context(), actor, id, audit.ActionWorkspaceSave, "workspace", id, "update "+body.Name, err)
	if httpx.WriteSvcErr(w, err) {
		return
	}
	httpx.WriteJSON(w, http.StatusOK, ws)
}

// Export 下载空间工程定义（JSON）。
func (c *Controller) Export(w http.ResponseWriter, r *http.Request) {
	actor := middleware.ActorFrom(r)
	id := r.PathValue("id")
	out, err := c.workspaces.Export(r.Context(), actor, id)
	c.audit.Note(r.Context(), actor, id, audit.ActionWorkspaceExport, "workspace", id, "export workspace", err)
	if httpx.WriteSvcErr(w, err) {
		return
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}

// Import 从 v1/v2 JSON 包新建空间或替换目标空间工程。
func (c *Controller) Import(w http.ResponseWriter, r *http.Request) {
	var req workspace.ImportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid body")
		return
	}
	actor := middleware.ActorFrom(r)
	out, err := c.workspaces.Import(r.Context(), actor, req)
	c.audit.Note(r.Context(), actor, out.RecID, audit.ActionWorkspaceImport, "workspace", out.RecID, "import "+req.Mode, err)
	if httpx.WriteSvcErr(w, err) {
		return
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}

// Clone 复制空间及其工程。
func (c *Controller) Clone(w http.ResponseWriter, r *http.Request) {
	var req workspace.CloneRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid body")
		return
	}
	actor := middleware.ActorFrom(r)
	out, err := c.workspaces.Clone(r.Context(), actor, req)
	c.audit.Note(r.Context(), actor, out.RecID, audit.ActionWorkspaceClone, "workspace", out.RecID, "clone from "+req.SourceRecID, err)
	if httpx.WriteSvcErr(w, err) {
		return
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}

// Delete 删除空间及其工程、环境与作业。
func (c *Controller) Delete(w http.ResponseWriter, r *http.Request) {
	actor := middleware.ActorFrom(r)
	id := r.PathValue("id")
	err := c.workspaces.Delete(r.Context(), actor, id)
	c.audit.Note(r.Context(), actor, id, audit.ActionWorkspaceDelete, "workspace", id, "delete workspace", err)
	if httpx.WriteSvcErr(w, err) {
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// ListMembers 列出空间成员。
func (c *Controller) ListMembers(w http.ResponseWriter, r *http.Request) {
	list, err := c.workspaces.ListMembers(r.Context(), middleware.ActorFrom(r), r.PathValue("id"))
	if httpx.WriteSvcErr(w, err) {
		return
	}
	httpx.WriteJSON(w, http.StatusOK, list)
}

// AddMember 添加或改角色。
func (c *Controller) AddMember(w http.ResponseWriter, r *http.Request) {
	var body struct {
		UserID string `json:"user_rec_id"`
		Role   string `json:"member_role"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	actor := middleware.ActorFrom(r)
	wsID := r.PathValue("id")
	err := c.workspaces.AddMember(r.Context(), actor, wsID, body.UserID, body.Role)
	c.audit.Note(r.Context(), actor, wsID, audit.ActionMemberAdd, "user", body.UserID, "add member as "+body.Role, err)
	if httpx.WriteSvcErr(w, err) {
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// RemoveMember 移除成员。
func (c *Controller) RemoveMember(w http.ResponseWriter, r *http.Request) {
	actor := middleware.ActorFrom(r)
	wsID := r.PathValue("id")
	userID := r.PathValue("userId")
	err := c.workspaces.RemoveMember(r.Context(), actor, wsID, userID)
	c.audit.Note(r.Context(), actor, wsID, audit.ActionMemberRemove, "user", userID, "remove member", err)
	if httpx.WriteSvcErr(w, err) {
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
