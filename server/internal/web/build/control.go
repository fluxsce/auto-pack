package build

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"auto-pack/server/internal/audit"
	"auto-pack/server/internal/batchlock"
	svc "auto-pack/server/internal/build"
	"auto-pack/server/internal/web/httpx"
	"auto-pack/server/internal/web/middleware"
)

// Controller 处理工作空间自动构建 HTTP，只做参数与状态码。
type Controller struct {
	svc   *svc.Service
	audit *audit.Service
}

// New 构造自动构建控制器。
func New(svc *svc.Service, audit *audit.Service) *Controller {
	return &Controller{svc: svc, audit: audit}
}

func (c *Controller) ready(w http.ResponseWriter) bool {
	if c.svc != nil {
		return true
	}
	httpx.WriteError(w, http.StatusServiceUnavailable, "build service unavailable")
	return false
}

func workspaceID(r *http.Request) string {
	return strings.TrimSpace(r.URL.Query().Get("workspace_rec_id"))
}

func requireWorkspace(w http.ResponseWriter, id string) bool {
	if id != "" {
		return true
	}
	httpx.WriteError(w, http.StatusBadRequest, "workspace_rec_id is required")
	return false
}

// Start 启动一次工作空间自动构建（202）。
func (c *Controller) Start(w http.ResponseWriter, r *http.Request) {
	if !c.ready(w) {
		return
	}
	var req svc.StartRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid body")
		return
	}
	actor := middleware.ActorFrom(r)
	out, err := c.svc.Start(r.Context(), actor, req)
	c.audit.Note(r.Context(), actor, req.WorkspaceRecID, audit.ActionJobSubmit, "workspace", req.WorkspaceRecID, "auto-build", err)
	if c.writeErr(w, err) {
		return
	}
	httpx.WriteJSON(w, http.StatusAccepted, out)
}

// Status 轮询当前自动构建进度。
func (c *Controller) Status(w http.ResponseWriter, r *http.Request) {
	if !c.ready(w) {
		return
	}
	wsID := workspaceID(r)
	if !requireWorkspace(w, wsID) {
		return
	}
	st, err := c.svc.Status(r.Context(), middleware.ActorFrom(r), wsID)
	if c.writeErr(w, err) {
		return
	}
	httpx.WriteJSON(w, http.StatusOK, st)
}

// History 列出空间自动构建记录。
func (c *Controller) History(w http.ResponseWriter, r *http.Request) {
	if !c.ready(w) {
		return
	}
	wsID := workspaceID(r)
	if !requireWorkspace(w, wsID) {
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	list, err := c.svc.History(r.Context(), middleware.ActorFrom(r), wsID, limit)
	if c.writeErr(w, err) {
		return
	}
	if list == nil {
		list = []svc.Record{}
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": list})
}

// Log 读取一次自动构建日志。
func (c *Controller) Log(w http.ResponseWriter, r *http.Request) {
	if !c.ready(w) {
		return
	}
	wsID := workspaceID(r)
	if !requireWorkspace(w, wsID) {
		return
	}
	offset, _ := strconv.ParseInt(r.URL.Query().Get("offset"), 10, 64)
	view, err := c.svc.Log(r.Context(), middleware.ActorFrom(r), wsID, r.PathValue("buildID"), offset)
	if c.writeErr(w, err) {
		return
	}
	httpx.WriteJSON(w, http.StatusOK, view)
}

func (c *Controller) writeErr(w http.ResponseWriter, err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, svc.ErrBuildInProgress) || errors.Is(err, svc.ErrNoProjects) || errors.Is(err, batchlock.ErrBusy) {
		httpx.WriteError(w, http.StatusConflict, err.Error())
		return true
	}
	return httpx.WriteSvcErr(w, err)
}
