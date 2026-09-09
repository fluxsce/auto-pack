package middleware

import (
	"context"
	"net/http"
	"strings"

	"auto-pack/server/internal/iam"
	"auto-pack/server/internal/token"
	"auto-pack/server/internal/web/httpx"
)

type ctxKey int

const actorKey ctxKey = 1

// RequireAuth injects iam.Actor from the Bearer token.
func RequireAuth(tokens *token.Issuer) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			raw := strings.TrimSpace(r.Header.Get("Authorization"))
			if !strings.HasPrefix(raw, "Bearer ") {
				httpx.WriteError(w, http.StatusUnauthorized, "missing bearer token")
				return
			}
			claims, err := tokens.Parse(strings.TrimSpace(strings.TrimPrefix(raw, "Bearer ")))
			if err != nil {
				httpx.WriteError(w, http.StatusUnauthorized, "invalid token")
				return
			}
			actor := iam.Actor{UserID: claims.UserRecID, LoginName: claims.LoginName}
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), actorKey, actor)))
		})
	}
}

// ActorFrom extracts the actor injected by RequireAuth.
func ActorFrom(r *http.Request) iam.Actor {
	v, _ := r.Context().Value(actorKey).(iam.Actor)
	return v
}
