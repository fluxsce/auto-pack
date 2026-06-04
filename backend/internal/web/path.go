package web

import "strings"

// BasePath is the URL prefix for API and SPA static files (reverse-proxy friendly).
const BasePath = "/autopack"

// APIPath builds a path under /autopack/api/...
func APIPath(parts ...string) string {
	b := strings.Builder{}
	b.WriteString(BasePath)
	b.WriteString("/api")
	for _, p := range parts {
		b.WriteByte('/')
		b.WriteString(p)
	}
	return b.String()
}
