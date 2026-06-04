package middleware

import (
	"context"
	"net/http"
	"strings"

	"auto-pack/backend/internal/httpx"
	authsvc "auto-pack/backend/internal/web/auth"
)

type contextKey int

const userRecIDKey contextKey = 1

// RequireAuth validates Bearer JWT and injects user rec_id into request context.
func RequireAuth(tokens *authsvc.TokenIssuer) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				httpx.WriteError(w, http.StatusUnauthorized, "missing authorization header")
				return
			}
			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
				httpx.WriteError(w, http.StatusUnauthorized, "invalid authorization header")
				return
			}
			claims, err := tokens.Parse(parts[1])
			if err != nil {
				httpx.WriteError(w, http.StatusUnauthorized, "invalid or expired token")
				return
			}
			ctx := context.WithValue(r.Context(), userRecIDKey, claims.UserRecID) //nolint:staticcheck // context key scoped to this package
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RecIDFromContext returns the authenticated user rec_id.
func RecIDFromContext(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(userRecIDKey).(string)
	return id, ok
}
