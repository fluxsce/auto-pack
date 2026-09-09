package operationlog

import (
	"net/http"
	"strconv"
	"strings"

	"auto-pack/backend/internal/httpx"
	"auto-pack/backend/internal/repository"
)

// Handler serves /api/operation-logs (read-only).
type Handler struct {
	repo *repository.OperationLogRepository
}

func NewHandler(repo *repository.OperationLogRepository) *Handler {
	return &Handler{repo: repo}
}

// Register mounts operation log routes.
func Register(mux *http.ServeMux, h *Handler, requireAuth func(http.Handler) http.Handler, apiBase string) {
	base := apiBase + "/operation-logs"
	mux.Handle("GET "+base, requireAuth(http.HandlerFunc(h.list)))
	mux.Handle("GET "+apiBase+"/operation-log/modules", requireAuth(http.HandlerFunc(h.modules)))
	mux.Handle("GET "+apiBase+"/operation-log/actions", requireAuth(http.HandlerFunc(h.actions)))
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	limit, _ := strconv.Atoi(q.Get("limit"))
	offset, _ := strconv.Atoi(q.Get("offset"))
	filter := repository.OperationLogListFilter{
		Module:    strings.TrimSpace(q.Get("module")),
		Action:    strings.TrimSpace(q.Get("action")),
		UserRecID: strings.TrimSpace(q.Get("user_rec_id")),
		Keyword:   strings.TrimSpace(q.Get("keyword")),
		Limit:     limit,
		Offset:    offset,
	}
	items, total, err := h.repo.List(filter)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"items":  items,
		"total":  total,
		"limit":  filter.Limit,
		"offset": filter.Offset,
	})
}

func (h *Handler) modules(w http.ResponseWriter, _ *http.Request) {
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"items": []string{"auth", "user", "env", "project", "workspace", "build"},
	})
}

func (h *Handler) actions(w http.ResponseWriter, _ *http.Request) {
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"items": []string{"create", "update", "delete", "clone", "login", "reset_password", "build", "verify"},
	})
}
