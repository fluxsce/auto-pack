// Package notes 解析并生成 Version.txt / UpdateNotes，以及发版提交说明。
package notes

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	relMavenVersionFile     = "src/main/resources/Version.txt"
	relMavenUpdateNotesFile = "src/main/resources/updatenotes/UpdateNotes.txt"
	relNodeVersionFile      = "src/resources/updateNotes/Version.txt"
	relNodeUpdateNotesFile  = "src/resources/updateNotes/updateNotes.txt"
)

// IsMavenReleaseProject reports whether project_type uses Maven-style resource paths.
func IsMavenReleaseProject(projectType string) bool {
	return strings.ToLower(strings.TrimSpace(projectType)) == "maven"
}

// IsNodeReleaseProject reports whether project_type uses Node/Vue-style resource paths.
func IsNodeReleaseProject(projectType string) bool {
	switch strings.ToLower(strings.TrimSpace(projectType)) {
	case "npm", "vue":
		return true
	default:
		return false
	}
}

// SupportsAutoReleaseVersionSync reports whether auto-release should sync Version/UpdateNotes.
func SupportsAutoReleaseVersionSync(projectType string) bool {
	return IsMavenReleaseProject(projectType) || IsNodeReleaseProject(projectType)
}

// ResolveResourcePaths returns canonical Version.txt and UpdateNotes.txt paths for projectType.
func ResolveResourcePaths(projectDir, projectType string) (versionPath, updateNotesPath string, ok bool) {
	projectDir = strings.TrimSpace(projectDir)
	if projectDir == "" {
		return "", "", false
	}
	if !SupportsAutoReleaseVersionSync(projectType) {
		return "", "", false
	}
	var relVersion, relNotes string
	if IsNodeReleaseProject(projectType) {
		relVersion = relNodeVersionFile
		relNotes = relNodeUpdateNotesFile
	} else {
		relVersion = relMavenVersionFile
		relNotes = relMavenUpdateNotesFile
	}
	versionPath = filepath.Join(projectDir, filepath.FromSlash(relVersion))
	updateNotesPath = filepath.Join(projectDir, filepath.FromSlash(relNotes))
	return versionPath, updateNotesPath, true
}

// ResourcePaths locates Version.txt and UpdateNotes.txt when both already exist.
func ResourcePaths(projectDir, projectType string) (versionPath, updateNotesPath string, ok bool) {
	versionPath, updateNotesPath, ok = ResolveResourcePaths(projectDir, projectType)
	if !ok {
		return "", "", false
	}
	if fileExists(versionPath) && fileExists(updateNotesPath) {
		return versionPath, updateNotesPath, true
	}
	return "", "", false
}

// EnsureResourceFiles creates missing Version.txt / UpdateNotes.txt (and parent dirs).
// It returns the paths that were created; the caller logs if needed.
func EnsureResourceFiles(versionPath, updateNotesPath string) ([]string, error) {
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
	}
	if !fileExists(updateNotesPath) {
		if err := os.WriteFile(updateNotesPath, []byte{}, 0o644); err != nil {
			return nil, fmt.Errorf("create UpdateNotes.txt: %w", err)
		}
		created = append(created, updateNotesPath)
	}
	return created, nil
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
