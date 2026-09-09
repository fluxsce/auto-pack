package me

import (
	"net/http"

	"auto-pack/backend/internal/httpx"
	authsvc "auto-pack/backend/internal/web/auth"
	"auto-pack/backend/internal/web/middleware"
)

// Handler handles GET /api/auth/me.
type Handler struct {
	svc *authsvc.Service
}

func NewHandler(svc *authsvc.Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	recID, ok := middleware.RecIDFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	u, err := h.svc.GetUser(recID)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	httpx.WriteJSON(w, http.StatusOK, u)
}
