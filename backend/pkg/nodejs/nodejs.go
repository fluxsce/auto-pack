package nodejs

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"auto-pack/backend/pkg/envtypes"
	"auto-pack/backend/pkg/probe"
	"auto-pack/backend/pkg/shell"
)

// Verify checks node/npm binaries.
func Verify(ctx context.Context, cfg envtypes.NodeJSConfig) probe.Result {
	details := map[string]string{}
	nodeBin, err := ResolveBinPath(cfg.NodePath, "node")
	if err != nil {
		return probe.Fail(err.Error(), details)
	}
	details["node_path"] = nodeBin

	res, err := shell.RunArgs(ctx, nodeBin, []string{"--version"}, shell.Options{})
	if err != nil {
		return probe.Fail(fmt.Sprintf("node --version failed: %v", err), details)
	}
	details["node_version"] = strings.TrimSpace(res.Stdout)

	if npmBin, err := ResolveBinPath(cfg.NpmPath, "npm"); err == nil {
		details["npm_path"] = npmBin
		if res, err := shell.RunArgs(ctx, npmBin, []string{"--version"}, shell.Options{}); err == nil {
			details["npm_version"] = strings.TrimSpace(res.Stdout)
		}
	}
	return probe.OK("Node.js available", details)
}

// ResolveBinPath resolves node_path/npm_path: command name, binary file, or install directory.
func ResolveBinPath(configured, name string) (string, error) {
	p := strings.TrimSpace(configured)
	if p == "" {
		found, err := exec.LookPath(name)
		if err != nil {
			return "", fmt.Errorf("%s not found in PATH", name)
		}
		return found, nil
	}
	resolved := normalizeBinPath(p, name)
	if !fileExists(resolved) {
		return "", fmt.Errorf("%s not found at %s", name, resolved)
	}
	return resolved, nil
}

// normalizeBinPath accepts a binary path or Node.js install directory.
func normalizeBinPath(p, name string) string {
	p = filepath.Clean(p)
	if info, err := os.Stat(p); err == nil && info.IsDir() {
		return joinBinInDir(p, name)
	}
	if os.PathSeparator == '\\' {
		lower := strings.ToLower(p)
		if !strings.HasSuffix(lower, ".exe") && !strings.HasSuffix(lower, ".cmd") {
			if _, err := os.Stat(p); os.IsNotExist(err) {
				for _, suffix := range []string{".exe", ".cmd"} {
					if fileExists(p + suffix) {
						return p + suffix
					}
				}
			}
		}
	}
	return p
}

func joinBinInDir(dir, name string) string {
	if os.PathSeparator == '\\' {
		for _, candidate := range windowsBinCandidates(name) {
			p := filepath.Join(dir, candidate)
			if fileExists(p) {
				return p
			}
		}
		return filepath.Join(dir, windowsBinCandidates(name)[0])
	}
	return filepath.Join(dir, name)
}

func windowsBinCandidates(name string) []string {
	switch name {
	case "node":
		return []string{"node.exe"}
	case "npm":
		return []string{"npm.cmd", "npm", "npm.exe"}
	case "npx":
		return []string{"npx.cmd", "npx", "npx.exe"}
	default:
		return []string{name + ".exe", name}
	}
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
