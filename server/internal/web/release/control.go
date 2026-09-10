package release

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"auto-pack/server/internal/audit"
	"auto-pack/server/internal/release"
	"auto-pack/server/internal/web/httpx"
	"auto-pack/server/internal/web/middleware"
)

// Controller 处理工作空间自动发版 HTTP，只做参数与状态码。
type Controller struct {
	svc   *release.Service
	audit *audit.Service
}

// New 构造发版控制器。
func New(svc *release.Service, audit *audit.Service) *Controller {
	return &Controller{svc: svc, audit: audit}
}

func (c *Controller) ready(w http.ResponseWriter) bool {
	if c.svc != nil {
		return true
	}
	httpx.WriteError(w, http.StatusServiceUnavailable, "release service unavailable")
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

// Start 启动一次工作空间自动发版（202）。
func (c *Controller) Start(w http.ResponseWriter, r *http.Request) {
	if !c.ready(w) {
		return
	}
	var req release.StartRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid body")
		return
	}
	actor := middleware.ActorFrom(r)
	out, err := c.svc.Start(r.Context(), actor, req)
	c.audit.Note(r.Context(), actor, req.WorkspaceRecID, audit.ActionJobSubmit, "workspace", req.WorkspaceRecID, "auto-release "+req.TagBranch, err)
	if c.writeErr(w, err) {
		return
	}
	httpx.WriteJSON(w, http.StatusAccepted, out)
}

// Status 轮询当前发版进度。
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

// Defaults 建议发版分支与下一 Tag。
func (c *Controller) Defaults(w http.ResponseWriter, r *http.Request) {
	if !c.ready(w) {
		return
	}
	wsID := workspaceID(r)
	if !requireWorkspace(w, wsID) {
		return
	}
	out, err := c.svc.Defaults(r.Context(), middleware.ActorFrom(r), wsID)
	if c.writeErr(w, err) {
		return
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}

// SavePostOptions 更新进行中发版的构建/推送选项。
func (c *Controller) SavePostOptions(w http.ResponseWriter, r *http.Request) {
	if !c.ready(w) {
		return
	}
	var in struct {
		WorkspaceRecID string              `json:"workspace_rec_id"`
		PostOptions    release.PostOptions `json:"post_options"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid body")
		return
	}
	actor := middleware.ActorFrom(r)
	recID, err := c.svc.SavePostOptions(r.Context(), actor, in.WorkspaceRecID, in.PostOptions)
	if c.writeErr(w, err) {
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"release_rec_id": recID, "status": "ok"})
}

// History 列出空间发版记录。
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
		list = []release.Record{}
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": list})
}

// Log 读取一次发版日志。
func (c *Controller) Log(w http.ResponseWriter, r *http.Request) {
	if !c.ready(w) {
		return
	}
	wsID := workspaceID(r)
	if !requireWorkspace(w, wsID) {
		return
	}
	offset, _ := strconv.ParseInt(r.URL.Query().Get("offset"), 10, 64)
	view, err := c.svc.Log(r.Context(), middleware.ActorFrom(r), wsID, r.PathValue("releaseID"), offset)
	if c.writeErr(w, err) {
		return
	}
	httpx.WriteJSON(w, http.StatusOK, view)
}

// SyncSources 串行预同步空间内 Git 工程。
// Accept: application/x-ndjson 时按工程推送进度；否则仍回整包 JSON。
func (c *Controller) SyncSources(w http.ResponseWriter, r *http.Request) {
	if !c.ready(w) {
		return
	}
	var in struct {
		WorkspaceRecID string `json:"workspace_rec_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid body")
		return
	}
	if !wantsSyncStream(r) {
		out, err := c.svc.SyncSources(r.Context(), middleware.ActorFrom(r), in.WorkspaceRecID)
		if c.writeErr(w, err) {
			return
		}
		httpx.WriteJSON(w, http.StatusOK, out)
		return
	}
	flusher, _ := w.(http.Flusher)
	w.Header().Set("Content-Type", "application/x-ndjson; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	enc := json.NewEncoder(w)
	writeEv := func(ev release.SyncEvent) {
		_ = enc.Encode(ev)
		if flusher != nil {
			flusher.Flush()
		}
	}
	_, err := c.svc.SyncSourcesProgress(r.Context(), middleware.ActorFrom(r), in.WorkspaceRecID, writeEv)
	if err != nil {
		writeEv(release.SyncEvent{Event: "error", Message: err.Error()})
	}
}

func wantsSyncStream(r *http.Request) bool {
	accept := strings.ToLower(r.Header.Get("Accept"))
	return strings.Contains(accept, "application/x-ndjson") || strings.TrimSpace(r.URL.Query().Get("stream")) == "1"
}

// MavenOrder 返回空间 Maven 依赖构建顺序。
func (c *Controller) MavenOrder(w http.ResponseWriter, r *http.Request) {
	if !c.ready(w) {
		return
	}
	wsID := workspaceID(r)
	if !requireWorkspace(w, wsID) {
		return
	}
	out, err := c.svc.MavenOrder(r.Context(), middleware.ActorFrom(r), wsID)
	if c.writeErr(w, err) {
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"analysis": out})
}

func (c *Controller) writeErr(w http.ResponseWriter, err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, release.ErrReleaseInProgress) || errors.Is(err, release.ErrSyncInProgress) || errors.Is(err, release.ErrNoActive) {
		httpx.WriteError(w, http.StatusConflict, err.Error())
		return true
	}
	return httpx.WriteSvcErr(w, err)
}
