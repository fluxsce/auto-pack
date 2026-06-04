package password

import (
	"encoding/json"
	"net/http"

	"auto-pack/backend/internal/httpx"
	authsvc "auto-pack/backend/internal/web/auth"
	"auto-pack/backend/internal/web/handler/shared"
	"auto-pack/backend/internal/web/middleware"
)

// Handler handles PUT /api/auth/password.
type Handler struct {
	svc *authsvc.Service
}

func NewHandler(svc *authsvc.Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	recID, ok := middleware.RecIDFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	var body struct {
		OldPassword string `json:"old_password"`
		NewPassword string `json:"new_password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.svc.ChangePassword(recID, body.OldPassword, body.NewPassword); err != nil {
		shared.WriteAuthServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"message": "password updated"})
}
