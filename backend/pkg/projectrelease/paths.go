package projectrelease

import (
	"os"
	"path/filepath"
	"strings"
)

const (
	relVersionFile     = "src/main/resources/Version.txt"
	relUpdateNotesFile = "src/main/resources/updatenotes/UpdateNotes.txt"
)

// ResourcePaths locates Version.txt and UpdateNotes.txt under a project work tree.
func ResourcePaths(projectDir string) (versionPath, updateNotesPath string, ok bool) {
	projectDir = strings.TrimSpace(projectDir)
	if projectDir == "" {
		return "", "", false
	}
	versionPath = filepath.Join(projectDir, filepath.FromSlash(relVersionFile))
	updateNotesPath = filepath.Join(projectDir, filepath.FromSlash(relUpdateNotesFile))
	if fileExists(versionPath) && fileExists(updateNotesPath) {
		return versionPath, updateNotesPath, true
	}
	return "", "", false
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
