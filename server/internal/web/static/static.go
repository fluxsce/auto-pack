package static

import (
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// Mount serves the SPA under prefix (e.g. /autopack/).
func Mount(mux *http.ServeMux, prefix, root string) error {
	root = filepath.Clean(root)
	if st, err := os.Stat(root); err != nil || !st.IsDir() {
		return err
	}
	fileServer := http.FileServer(http.Dir(root))
	mux.Handle(prefix, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rel := strings.TrimPrefix(r.URL.Path, strings.TrimSuffix(prefix, "/"))
		rel = strings.TrimPrefix(rel, "/")
		full := filepath.Join(root, filepath.FromSlash(rel))
		if rel != "" {
			if _, err := os.Stat(full); err == nil {
				http.StripPrefix(strings.TrimSuffix(prefix, "/"), fileServer).ServeHTTP(w, r)
				return
			}
		}
		http.ServeFile(w, r, filepath.Join(root, "index.html"))
	}))
	return nil
}

// ResolveRoot picks frontend dist.
func ResolveRoot(configured string) (string, error) {
	if configured != "" {
		return configured, nil
	}
	candidates := []string{"web", "../dist", "../../dist", "frontend/dist"}
	if exe, err := os.Executable(); err == nil {
		candidates = append([]string{filepath.Join(filepath.Dir(exe), "web")}, candidates...)
	}
	for _, c := range candidates {
		if st, err := os.Stat(filepath.Join(c, "index.html")); err == nil && !st.IsDir() {
			abs, _ := filepath.Abs(c)
			return abs, nil
		}
	}
	return "", fs.ErrNotExist
}
