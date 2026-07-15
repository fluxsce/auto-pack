package projectcfg

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"auto-pack/backend/internal/repository"
)

const (
	ScmModeLocal = "local"
	ScmModeGit   = "git"
)

// NormalizeScmMode returns local or git (default local).
func NormalizeScmMode(mode string) string {
	if strings.EqualFold(strings.TrimSpace(mode), ScmModeGit) {
		return ScmModeGit
	}
	return ScmModeLocal
}

// EffectiveScmMode reads scm_mode from config_json, with legacy inference from repo_url.
func EffectiveScmMode(project *repository.PackProject) string {
	if project == nil {
		return ScmModeLocal
	}
	cfg, err := ParseConfigJSON(project.ConfigJSON)
	if err == nil {
		mode := strings.TrimSpace(cfg.ScmMode)
		if mode == ScmModeGit || mode == ScmModeLocal {
			return mode
		}
	}
	if strings.TrimSpace(project.RepoURL) != "" {
		return ScmModeGit
	}
	return ScmModeLocal
}

// UsesGitSource reports whether builds should pull from a remote Git repository.
func UsesGitSource(project *repository.PackProject) bool {
	return EffectiveScmMode(project) == ScmModeGit
}

// LocalSourcePathFromConfig returns optional local path override from config_json.
func LocalSourcePathFromConfig(project *repository.PackProject) string {
	if project == nil {
		return ""
	}
	cfg, err := ParseConfigJSON(project.ConfigJSON)
	if err != nil {
		return ""
	}
	return NormalizeLocalSourcePath(cfg.LocalSourcePath)
}

// NormalizeLocalSourcePath trims a configured local source path.
func NormalizeLocalSourcePath(path string) string {
	path = strings.TrimSpace(path)
	path = strings.Trim(path, `"'`)
	return filepath.ToSlash(path)
}

// ResolveLocalSourceAbsDir resolves a path relative to workspace root or absolute.
func ResolveLocalSourceAbsDir(localPath, workspaceRoot string) string {
	localPath = NormalizeLocalSourcePath(localPath)
	if localPath == "" {
		return ""
	}
	if filepath.IsAbs(localPath) {
		return filepath.Clean(localPath)
	}
	workspaceRoot = strings.TrimSpace(workspaceRoot)
	if workspaceRoot == "" {
		return filepath.Clean(localPath)
	}
	return filepath.Clean(filepath.Join(workspaceRoot, filepath.FromSlash(localPath)))
}

// WorkspaceRootForProject returns the workspace filesystem root.
func WorkspaceRootForProject(ws *repository.PackWorkspace, dataDir string) string {
	if ws == nil {
		return ""
	}
	return ResolveWorkspaceRoot(ws.RootDir, dataDir, ws.WorkspaceCode)
}

// ResolveProjectSourceAbsDir is the directory used for builds and local-source Git push.
func ResolveProjectSourceAbsDir(
	project *repository.PackProject,
	ws *repository.PackWorkspace,
	dataDir string,
) string {
	if project == nil || ws == nil {
		return ""
	}
	wsRoot := WorkspaceRootForProject(ws, dataDir)
	if override := LocalSourcePathFromConfig(project); override != "" {
		if abs := ResolveLocalSourceAbsDir(override, wsRoot); abs != "" {
			return abs
		}
	}
	return JoinWorkDir(ws.RootDir, dataDir, ws.WorkspaceCode, project.WorkDir)
}

// ValidateScmConfig checks scm_mode against repository fields.
func ValidateScmConfig(cfg Config, repoURL string) error {
	mode := NormalizeScmMode(cfg.ScmMode)
	if mode == ScmModeGit && strings.TrimSpace(repoURL) == "" {
		return errors.New("repository URL is required for Git source mode")
	}
	if mode == ScmModeLocal {
		if path := NormalizeLocalSourcePath(cfg.LocalSourcePath); path != "" {
			if strings.Contains(path, "..") {
				return errors.New("local source path must not contain ..")
			}
		}
	}
	return nil
}

// LocalSourceDirReady verifies the resolved source directory exists.
func LocalSourceDirReady(absDir string) error {
	absDir = strings.TrimSpace(absDir)
	if absDir == "" {
		return errors.New("local source path is empty")
	}
	info, err := os.Stat(absDir)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return errors.New("local source path is not a directory")
	}
	return nil
}

// HasRemotePush reports whether remote Git push is configured.
func HasRemotePush(project *repository.PackProject) bool {
	return project != nil && strings.TrimSpace(project.RepoURL) != ""
}
