package nodejs

import (
	"encoding/base64"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"auto-pack/backend/pkg/envtypes"
)

// ResolveNpmrcPath returns a cleaned .npmrc path when it exists.
func ResolveNpmrcPath(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	raw = os.ExpandEnv(raw)
	if strings.HasPrefix(raw, "~") {
		if home, err := os.UserHomeDir(); err == nil && home != "" {
			raw = filepath.Join(home, strings.TrimPrefix(raw, "~"))
		}
	}
	raw = filepath.Clean(raw)
	info, err := os.Stat(raw)
	if err != nil || info.IsDir() {
		return ""
	}
	return raw
}

// NpmrcCandidates lists common .npmrc locations that exist on this host.
func NpmrcCandidates() []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, 2)
	add := func(p string) {
		p = strings.TrimSpace(p)
		if p == "" {
			return
		}
		p = os.ExpandEnv(p)
		if strings.HasPrefix(p, "~") {
			if home, err := os.UserHomeDir(); err == nil && home != "" {
				p = filepath.Join(home, strings.TrimPrefix(p, "~"))
			}
		}
		p = filepath.Clean(p)
		if _, ok := seen[p]; ok {
			return
		}
		seen[p] = struct{}{}
		if ResolveNpmrcPath(p) != "" {
			out = append(out, p)
		}
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		add(filepath.Join(home, ".npmrc"))
	}
	return out
}

// HasNpmUserConfig reports whether npm/pnpm settings should be injected at build time.
func HasNpmUserConfig(cfg envtypes.NodeJSConfig) bool {
	if PnpmAllowAllBuildsEnabled(cfg) {
		return true
	}
	if ResolveNpmrcPath(cfg.NpmrcPath) != "" {
		return true
	}
	registry := strings.TrimSpace(cfg.Registry)
	if registry == "" {
		return false
	}
	if strings.TrimSpace(cfg.AuthToken) != "" {
		return true
	}
	if strings.TrimSpace(cfg.AuthUsername) != "" {
		return true
	}
	return false
}

// PnpmAllowAllBuildsEnabled reports whether pnpm should run all dependency install scripts.
// Defaults to true when unset so internal CI builds work without per-project approve-builds.
func PnpmAllowAllBuildsEnabled(cfg envtypes.NodeJSConfig) bool {
	if cfg.PnpmAllowAllBuilds == nil {
		return true
	}
	return *cfg.PnpmAllowAllBuilds
}

// PrepareUserConfig returns an npm userconfig path for NPM_CONFIG_USERCONFIG and an optional cleanup func.
func PrepareUserConfig(cfg envtypes.NodeJSConfig, dataDir string) (path string, cleanup func(), summary string, err error) {
	noCleanup := func() {}
	resolved := ResolveNpmrcPath(cfg.NpmrcPath)
	overlay := renderUserConfigOverlay(cfg)

	if resolved != "" && overlay == "" {
		return resolved, noCleanup, "npmrc=" + resolved, nil
	}

	base := ""
	if resolved != "" {
		data, readErr := os.ReadFile(resolved)
		if readErr != nil {
			return "", noCleanup, "", fmt.Errorf("read npmrc %s: %w", resolved, readErr)
		}
		base = string(data)
	}

	content := mergeNpmrcContent(base, overlay)
	if strings.TrimSpace(content) == "" {
		return "", noCleanup, "", nil
	}

	dir := filepath.Join(strings.TrimSpace(dataDir), "npmrc")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", noCleanup, "", fmt.Errorf("mkdir npmrc temp dir: %w", err)
	}
	f, err := os.CreateTemp(dir, "build-*.npmrc")
	if err != nil {
		return "", noCleanup, "", fmt.Errorf("create npmrc temp file: %w", err)
	}
	tempPath := f.Name()
	if _, err := f.WriteString(content); err != nil {
		_ = f.Close()
		_ = os.Remove(tempPath)
		return "", noCleanup, "", fmt.Errorf("write npmrc temp file: %w", err)
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tempPath)
		return "", noCleanup, "", err
	}
	summary = "npm userconfig (merged)"
	if PnpmAllowAllBuildsEnabled(cfg) {
		summary += ", pnpm allow all builds"
	}
	return tempPath, func() { _ = os.Remove(tempPath) }, summary, nil
}

func renderUserConfigOverlay(cfg envtypes.NodeJSConfig) string {
	var blocks []string
	if PnpmAllowAllBuildsEnabled(cfg) {
		blocks = append(blocks, "dangerously-allow-all-builds=true")
	}
	if auth := renderRegistryAuthBlock(cfg); auth != "" {
		blocks = append(blocks, auth)
	}
	return strings.Join(blocks, "\n")
}

func mergeNpmrcContent(base, overlay string) string {
	base = strings.TrimSpace(base)
	overlay = strings.TrimSpace(overlay)
	switch {
	case base == "":
		return overlay
	case overlay == "":
		return base
	default:
		return base + "\n" + overlay + "\n"
	}
}

func renderRegistryAuthBlock(cfg envtypes.NodeJSConfig) string {
	registry := strings.TrimSpace(cfg.Registry)
	if registry == "" {
		return ""
	}
	key := registryAuthKey(registry)
	if key == "" {
		return ""
	}

	var lines []string
	lines = append(lines, "registry="+quoteNpmrcValue(registry))

	token := strings.TrimSpace(cfg.AuthToken)
	user := strings.TrimSpace(cfg.AuthUsername)
	pass := cfg.AuthPassword
	switch {
	case token != "":
		lines = append(lines, key+":_authToken="+token)
	case user != "":
		lines = append(lines, key+":username="+user)
		lines = append(lines, key+":_password="+base64.StdEncoding.EncodeToString([]byte(pass)))
	default:
		return strings.Join(lines, "\n")
	}
	lines = append(lines, key+":always-auth=true")
	return strings.Join(lines, "\n")
}

func registryAuthKey(registry string) string {
	registry = strings.TrimSpace(registry)
	registry = strings.TrimSuffix(registry, "/")
	u, err := url.Parse(registry)
	if err != nil || u.Host == "" {
		return ""
	}
	path := u.Path
	if path == "" {
		path = "/"
	}
	if !strings.HasSuffix(path, "/") {
		path += "/"
	}
	return "//" + u.Host + path
}

// EnsureWorkdirPnpmAllowBuilds appends dangerously-allow-all-builds to the project .npmrc when missing.
func EnsureWorkdirPnpmAllowBuilds(workDir string) error {
	workDir = strings.TrimSpace(workDir)
	if workDir == "" {
		return nil
	}
	path := filepath.Join(workDir, ".npmrc")
	const marker = "dangerously-allow-all-builds=true"
	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("read project .npmrc: %w", err)
	}
	content := string(data)
	if strings.Contains(strings.ToLower(content), "dangerously-allow-all-builds") {
		return nil
	}
	if strings.TrimSpace(content) != "" && !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	content += marker + "\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write project .npmrc: %w", err)
	}
	return nil
}

func quoteNpmrcValue(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return `""`
	}
	if strings.ContainsAny(v, " \t#") {
		return `"` + strings.ReplaceAll(v, `"`, `\"`) + `"`
	}
	return v
}
