package projectrelease

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	relVersionFile     = "src/main/resources/Version.txt"
	relUpdateNotesFile = "src/main/resources/updatenotes/UpdateNotes.txt"
)

// ResolveResourcePaths returns canonical Version.txt and UpdateNotes.txt paths under projectDir.
func ResolveResourcePaths(projectDir string) (versionPath, updateNotesPath string, ok bool) {
	projectDir = strings.TrimSpace(projectDir)
	if projectDir == "" {
		return "", "", false
	}
	versionPath = filepath.Join(projectDir, filepath.FromSlash(relVersionFile))
	updateNotesPath = filepath.Join(projectDir, filepath.FromSlash(relUpdateNotesFile))
	return versionPath, updateNotesPath, true
}

// ResourcePaths locates Version.txt and UpdateNotes.txt when both already exist.
func ResourcePaths(projectDir string) (versionPath, updateNotesPath string, ok bool) {
	versionPath, updateNotesPath, ok = ResolveResourcePaths(projectDir)
	if !ok {
		return "", "", false
	}
	if fileExists(versionPath) && fileExists(updateNotesPath) {
		return versionPath, updateNotesPath, true
	}
	return "", "", false
}

// EnsureResourceFiles creates missing Version.txt / UpdateNotes.txt (and parent dirs). Returns created paths.
func EnsureResourceFiles(versionPath, updateNotesPath string, log *publishLogger) ([]string, error) {
	versionPath = strings.TrimSpace(versionPath)
	updateNotesPath = strings.TrimSpace(updateNotesPath)
	if versionPath == "" || updateNotesPath == "" {
		return nil, fmt.Errorf("resource paths are empty")
	}
	var created []string
	if err := os.MkdirAll(filepath.Dir(versionPath), 0o755); err != nil {
		return nil, fmt.Errorf("mkdir Version.txt parent: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(updateNotesPath), 0o755); err != nil {
		return nil, fmt.Errorf("mkdir UpdateNotes parent: %w", err)
	}
	if !fileExists(versionPath) {
		if err := os.WriteFile(versionPath, []byte{}, 0o644); err != nil {
			return nil, fmt.Errorf("create Version.txt: %w", err)
		}
		created = append(created, versionPath)
		if log != nil {
			log.step("已自动创建 Version.txt")
		}
	}
	if !fileExists(updateNotesPath) {
		if err := os.WriteFile(updateNotesPath, []byte{}, 0o644); err != nil {
			return nil, fmt.Errorf("create UpdateNotes.txt: %w", err)
		}
		created = append(created, updateNotesPath)
		if log != nil {
			log.step("已自动创建 UpdateNotes.txt")
		}
	}
	return created, nil
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
