package static

import (
	"fmt"
	"os"
	"path/filepath"

	"auto-pack/backend/internal/web/assets"
)

// ResolveRoot returns the on-disk frontend dist directory (index.html + assets/).
// explicit: web.frontend.path from configs/app.yaml; otherwise {exe}/web, then ../frontend/dist for dev.
func ResolveRoot(explicit string) (string, error) {
	if explicit != "" {
		if st, err := os.Stat(explicit); err == nil && st.IsDir() {
			return filepath.Clean(explicit), nil
		}
		return "", fmt.Errorf("web.frontend.path is not a directory: %s", explicit)
	}
	if exe, err := os.Executable(); err == nil {
		p := filepath.Join(filepath.Dir(exe), assets.ReleaseWebDir)
		if st, err := os.Stat(p); err == nil && st.IsDir() {
			return filepath.Clean(p), nil
		}
	}
	if st, err := os.Stat(assets.DevDistPath); err == nil && st.IsDir() {
		return filepath.Clean(assets.DevDistPath), nil
	}
	return "", fmt.Errorf("frontend dist not found (set web.frontend.path in configs/app.yaml, or place %s beside binary, or build %s)",
		assets.ReleaseWebDir, assets.DevDistPath)
}
