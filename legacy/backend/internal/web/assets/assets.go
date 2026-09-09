package assets

import (
	"os"
	"path/filepath"
)

// ReleaseWebDir is the directory name (next to the autopack binary) holding frontend dist files.
const ReleaseWebDir = "web"

// DevDistPath is the relative path from backend/ to frontend dist when running go run locally.
const DevDistPath = "../frontend/dist"

// EnsureReleaseWebDir creates the release web directory under stagingRoot (package output dir).
func EnsureReleaseWebDir(stagingRoot string) (string, error) {
	p := filepath.Join(stagingRoot, ReleaseWebDir)
	return p, os.MkdirAll(p, 0o755)
}
