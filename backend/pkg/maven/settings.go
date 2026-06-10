package maven

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

var mvnInjectPattern = regexp.MustCompile(`(?i)\bmvn\b`)

// ResolveSettingsPath returns a cleaned settings.xml path when it exists.
func ResolveSettingsPath(raw string) string {
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

// SettingsCandidates lists common settings.xml locations that exist on this host.
func SettingsCandidates(mavenHome string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, 4)
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
		if ResolveSettingsPath(p) != "" {
			out = append(out, p)
		}
	}

	if home, err := os.UserHomeDir(); err == nil && home != "" {
		add(filepath.Join(home, ".m2", "settings.xml"))
	}
	mavenHome = strings.TrimSpace(os.ExpandEnv(mavenHome))
	if mavenHome == "" {
		mavenHome = strings.TrimSpace(os.Getenv("MAVEN_HOME"))
	}
	if mavenHome != "" {
		add(filepath.Join(mavenHome, "conf", "settings.xml"))
	}
	return out
}

// InjectSettingsIntoScript adds `-s <settings>` to bare `mvn` invocations when not already present.
func InjectSettingsIntoScript(script, settingsPath string) string {
	settingsPath = ResolveSettingsPath(settingsPath)
	if settingsPath == "" || strings.TrimSpace(script) == "" {
		return script
	}
	lower := strings.ToLower(script)
	if strings.Contains(lower, "--settings") || strings.Contains(lower, " -s ") {
		return script
	}
	quoted := strconv.Quote(settingsPath)
	return mvnInjectPattern.ReplaceAllString(script, "mvn -s "+quoted)
}
