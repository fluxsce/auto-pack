package httpx

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"runtime/debug"
	"strings"

	"auto-pack/server/internal/iam"
)

// WriteJSON writes a JSON response.
func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// WriteError writes {"error": message}，4xx/5xx（除未登录）同时写入进程日志。
func WriteError(w http.ResponseWriter, status int, message string) {
	if status >= 400 && status != http.StatusUnauthorized && strings.TrimSpace(message) != "" {
		log.Printf("http %d: %s", status, message)
	}
	WriteJSON(w, status, map[string]string{"error": message})
}

// WriteIAM maps iam errors to HTTP status.
func WriteIAM(w http.ResponseWriter, err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, iam.ErrUnauthenticated) {
		WriteError(w, http.StatusUnauthorized, err.Error())
		return true
	}
	if errors.Is(err, iam.ErrForbidden) {
		WriteError(w, http.StatusForbidden, err.Error())
		return true
	}
	return false
}

// WriteSvcErr maps a service error to an HTTP status. Returns true when err != nil.
func WriteSvcErr(w http.ResponseWriter, err error) bool {
	if err == nil {
		return false
	}
	if WriteIAM(w, err) {
		return true
	}
	msg := err.Error()
	status := http.StatusBadRequest
	if strings.Contains(msg, "not found") || strings.Contains(strings.ToLower(msg), "sql: no rows") {
		status = http.StatusNotFound
	}
	WriteError(w, status, msg)
	return true
}

// WithRecover 拦住未处理 panic，写入进程日志并回 500，避免拖垮整个进程。
func WithRecover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				log.Printf("panic %s %s: %v\n%s", r.Method, r.URL.Path, rec, debug.Stack())
				WriteError(w, http.StatusInternalServerError, "internal error")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// WithCORS wraps a handler with permissive CORS for the console.
func WithCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", r.Header.Get("Origin"))
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
