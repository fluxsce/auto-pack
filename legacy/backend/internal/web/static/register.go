package static

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"auto-pack/backend/internal/httpx"
	"auto-pack/backend/pkg/logger"
)

// Mount registers Vue SPA static files (gateway-style): prefix/assets, favicon, index, SPA fallback.
// prefix is typically /autopack; root is the frontend dist directory.
func Mount(mux *http.ServeMux, prefix, root string) error {
	root = filepath.Clean(root)
	indexPath := filepath.Join(root, "index.html")
	if st, err := os.Stat(indexPath); err != nil || st.IsDir() {
		return fmt.Errorf("index.html not found in %s: %w", root, err)
	}

	prefix = strings.TrimSuffix(prefix, "/")
	if prefix == "" {
		prefix = "/"
	}

	assetsDir := filepath.Join(root, "assets")
	if st, err := os.Stat(assetsDir); err == nil && st.IsDir() {
		urlPrefix := prefix + "/assets/"
		mux.Handle(urlPrefix, http.StripPrefix(urlPrefix, http.FileServer(http.Dir(assetsDir))))
		logger.Info("frontend assets mounted", "url", urlPrefix, "dir", assetsDir)
	}

	faviconPath := filepath.Join(root, "favicon.ico")
	if st, err := os.Stat(faviconPath); err == nil && !st.IsDir() {
		mux.HandleFunc(prefix+"/favicon.ico", func(w http.ResponseWriter, r *http.Request) {
			http.ServeFile(w, r, faviconPath)
		})
	}

	serveIndex := func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		http.ServeFile(w, r, indexPath)
	}

	mux.HandleFunc(prefix, serveIndex)
	mux.HandleFunc(prefix+"/", spaFallback(prefix, root, indexPath))

	logger.Info("frontend SPA mounted", "prefix", prefix, "root", root)
	return nil
}

// spaFallback mirrors gateway NoRoute: API under prefix -> JSON 404; other paths -> index.html.
func spaFallback(prefix, root, indexPath string) http.HandlerFunc {
	apiPrefix := prefix + "/api/"
	return func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

		if strings.HasPrefix(path, apiPrefix) {
			httpx.WriteError(w, http.StatusNotFound, "API endpoint not found")
			return
		}

		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		rel := strings.TrimPrefix(path, prefix)
		rel = strings.TrimPrefix(rel, "/")
		if rel != "" && rel != "." {
			fpath := filepath.Join(root, filepath.FromSlash(rel))
			if st, err := os.Stat(fpath); err == nil && !st.IsDir() {
				http.ServeFile(w, r, fpath)
				return
			}
		}

		http.ServeFile(w, r, indexPath)
	}
}
