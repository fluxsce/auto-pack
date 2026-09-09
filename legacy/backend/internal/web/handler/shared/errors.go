package shared

import (
	"errors"
	"net/http"

	"auto-pack/backend/internal/httpx"
	authsvc "auto-pack/backend/internal/web/auth"
)

// WriteAuthServiceError maps auth service errors to HTTP responses.
func WriteAuthServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, authsvc.ErrInvalidCredentials):
		httpx.WriteError(w, http.StatusUnauthorized, err.Error())
	case errors.Is(err, authsvc.ErrUsernameTaken):
		httpx.WriteError(w, http.StatusConflict, err.Error())
	case errors.Is(err, authsvc.ErrWeakPassword):
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, authsvc.ErrAccountDisabled):
		httpx.WriteError(w, http.StatusForbidden, err.Error())
	default:
		httpx.WriteError(w, http.StatusInternalServerError, err.Error())
	}
}
