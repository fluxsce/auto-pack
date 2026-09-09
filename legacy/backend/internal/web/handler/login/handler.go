package login

import (
	"encoding/json"
	"net/http"
	"strings"

	"auto-pack/backend/internal/httpx"
	authsvc "auto-pack/backend/internal/web/auth"
	"auto-pack/backend/internal/web/handler/shared"
	"auto-pack/backend/pkg/audit"
)

// Handler handles POST /api/auth/login.
type Handler struct {
	svc   *authsvc.Service
	audit *audit.Recorder
}

func NewHandler(svc *authsvc.Service, rec *audit.Recorder) *Handler {
	return &Handler{svc: svc, audit: rec}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	loginName := strings.TrimSpace(body.Username)
	result, err := h.svc.Login(loginName, body.Password)
	if err != nil {
		h.audit.Record(r.Context(), r, audit.Entry{
			Action:       audit.ActionLogin,
			Module:       audit.ModuleAuth,
			TargetType:   "user",
			TargetName:   loginName,
			Summary:      "登录失败: " + loginName,
			Success:      false,
			ErrorMessage: err.Error(),
			LoginName:    loginName,
		})
		shared.WriteAuthServiceError(w, err)
		return
	}
	h.audit.Record(r.Context(), r, audit.Entry{
		Action:     audit.ActionLogin,
		Module:     audit.ModuleAuth,
		TargetType: "user",
		TargetID:   result.User.RecID,
		TargetName: result.User.LoginName,
		Summary:    "登录成功: " + result.User.LoginName,
		Success:    true,
		UserRecID:  result.User.RecID,
		LoginName:  result.User.LoginName,
	})
	httpx.WriteJSON(w, http.StatusOK, result)
}
