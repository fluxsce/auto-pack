package audit

import (
	"net/http"
	"strconv"

	"auto-pack/server/internal/audit"
	"auto-pack/server/internal/web/httpx"
	"auto-pack/server/internal/web/middleware"
)

// Controller 处理审计查询 HTTP。
type Controller struct {
	audit *audit.Service
}

// New 构造审计控制器。
func New(svc *audit.Service) *Controller {
	return &Controller{audit: svc}
}

// List 列出空间内审计记录。
func (c *Controller) List(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	page, _ := strconv.Atoi(q.Get("page"))
	pageSize, _ := strconv.Atoi(q.Get("page_size"))
	out, err := c.audit.List(r.Context(), middleware.ActorFrom(r), audit.ListQuery{
		WorkspaceID: q.Get("workspace_rec_id"),
		Action:      q.Get("action_code"),
		Result:      q.Get("result"),
		Keyword:     q.Get("q"),
		Page:        page,
		PageSize:    pageSize,
	})
	if httpx.WriteSvcErr(w, err) {
		return
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}
