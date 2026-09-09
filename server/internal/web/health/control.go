package health

import (
	"net/http"
	"time"

	"auto-pack/server/internal/config"
	"auto-pack/server/internal/web/httpx"
)

// Controller 处理探活 HTTP。
type Controller struct {
	startedAt time.Time
}

// New 构造探活控制器。
func New(startedAt time.Time) *Controller {
	return &Controller{startedAt: startedAt}
}

// Get 返回进程状态与控制台版本。
func (c *Controller) Get(w http.ResponseWriter, _ *http.Request) {
	httpx.WriteJSON(w, http.StatusOK, map[string]string{
		"status":     "ok",
		"version":    config.AppVersion,
		"started_at": c.startedAt.Format(time.RFC3339Nano),
	})
}
