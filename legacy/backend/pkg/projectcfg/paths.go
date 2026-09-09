package projectcfg

import (
	"path/filepath"
	"strings"
)

const defaultWorkspaceCode = "default"

// WorkspaceDataSubdir is the folder under application data/ for per-workspace roots
// when AP_PACK_WORKSPACE.root_dir is empty.
const WorkspaceDataSubdir = "workspaces"

// SuggestWorkDir builds the relative project directory under the workspace root
// (or data/workspaces/{code} when workspace root_dir is empty): {slug(project_name)}.
func SuggestWorkDir(projectName string) string {
	code := SlugFromProjectName(projectName)
	if code == "" {
		return ""
	}
	return filepath.ToSlash(code)
}

// ResolveWorkspaceRoot returns root_dir when set; otherwise data/workspaces/{workspaceCode}.
func ResolveWorkspaceRoot(rootDir, dataDir, workspaceCode string) string {
	if strings.TrimSpace(rootDir) != "" {
		return filepath.Clean(rootDir)
	}
	code := strings.TrimSpace(workspaceCode)
	if code == "" {
		code = defaultWorkspaceCode
	}
	return filepath.Clean(filepath.Join(dataDir, WorkspaceDataSubdir, code))
}

// JoinWorkDir combines workspace root with relative work_dir under the project workspace.
func JoinWorkDir(rootDir, dataDir, workspaceCode, workDir string) string {
	workDir = strings.TrimSpace(workDir)
	if workDir == "" {
		return ""
	}
	if filepath.IsAbs(workDir) {
		return filepath.Clean(workDir)
	}
	root := ResolveWorkspaceRoot(rootDir, dataDir, workspaceCode)
	return filepath.Clean(filepath.Join(root, workDir))
}
