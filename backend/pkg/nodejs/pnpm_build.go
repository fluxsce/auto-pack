package nodejs

import (
	"regexp"
	"strings"
)

var pnpmAllowBuildsCmd = regexp.MustCompile(`(?i)\bpnpm\s+(install|i|ci|add)\b`)

// AppendPnpmAllowAllBuildsEnv adds env vars pnpm reads for dangerouslyAllowAllBuilds.
func AppendPnpmAllowAllBuildsEnv(env []string) []string {
	// pnpm/npm config from env; lowercase preferred inside shell scripts.
	pairs := []string{
		"npm_config_dangerously_allow_all_builds=true",
		"NPM_CONFIG_DANGEROUSLY_ALLOW_ALL_BUILDS=true",
	}
	out := append([]string{}, env...)
	for _, p := range pairs {
		if !hasEnvKey(out, strings.SplitN(p, "=", 2)[0]) {
			out = append(out, p)
		}
	}
	return out
}

func hasEnvKey(env []string, key string) bool {
	key = strings.ToLower(strings.TrimSpace(key))
	for _, e := range env {
		if i := strings.IndexByte(e, '='); i > 0 {
			if strings.ToLower(strings.TrimSpace(e[:i])) == key {
				return true
			}
		}
	}
	return false
}

// InjectPnpmAllowAllBuilds adds --dangerously-allow-all-builds to pnpm install/add/ci only (not run).
func InjectPnpmAllowAllBuilds(script string) string {
	script = strings.TrimSpace(script)
	if script == "" {
		return script
	}
	if strings.Contains(strings.ToLower(script), "dangerously-allow-all-builds") {
		return script
	}
	return pnpmAllowBuildsCmd.ReplaceAllString(script, "$0 --dangerously-allow-all-builds")
}
