package env

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// KindConfig returns decrypted config for the first enabled env of kind, or nil.
func (s *Service) KindConfig(ctx context.Context, workspaceID, kind string) map[string]any {
	if s == nil {
		return nil
	}
	_, cfg, err := s.FindKind(ctx, workspaceID, kind)
	if err != nil || len(cfg) == 0 {
		return nil
	}
	return cfg
}

// ProcEnv builds extra process env from JDK / Maven / Node workspace envs.
func ProcEnv(jdk, mavenCfg, node map[string]any) []string {
	var out []string
	var pathDirs []string

	javaHome := mapString(jdk, "java_home")
	if javaHome == "" && jdk != nil {
		javaHome = mapString(mavenCfg, "java_home")
	}
	if javaHome != "" {
		out = append(out, "JAVA_HOME="+javaHome)
		pathDirs = append(pathDirs, filepath.Join(javaHome, "bin"))
	}
	if p := binDir(mapString(jdk, "java_path")); p != "" {
		pathDirs = append(pathDirs, p)
	}

	if home := mapString(mavenCfg, "maven_home"); home != "" {
		out = append(out, "MAVEN_HOME="+home, "M2_HOME="+home)
		pathDirs = append(pathDirs, filepath.Join(home, "bin"))
	}
	if p := binDir(mapString(mavenCfg, "mvn_path")); p != "" {
		pathDirs = append(pathDirs, p)
	}

	if p := binDir(mapString(node, "node_path")); p != "" {
		pathDirs = append(pathDirs, p)
	}
	if p := binDir(mapString(node, "npm_path")); p != "" {
		pathDirs = append(pathDirs, p)
	}
	if p := binDir(mapString(node, "npx_path")); p != "" {
		pathDirs = append(pathDirs, p)
	}
	if registry := mapString(node, "registry"); registry != "" {
		out = append(out, "npm_config_registry="+registry, "NPM_CONFIG_REGISTRY="+registry)
	}
	if token := mapString(node, "auth_token"); token != "" {
		out = append(out, "NODE_AUTH_TOKEN="+token, "NPM_TOKEN="+token)
	}
	if npmrc := mapString(node, "npmrc_path"); npmrc != "" {
		out = append(out, "NPM_CONFIG_USERCONFIG="+npmrc)
	}
	if node != nil && node["pnpm_allow_all_builds"] != false {
		out = append(out, "pnpm_config_ignore_scripts=false")
	}
	if user := mapString(node, "auth_username"); user != "" {
		out = append(out, "NPM_CONFIG_USERNAME="+user)
	}
	if pass := mapString(node, "auth_password"); pass != "" {
		out = append(out, "NPM_CONFIG_PASSWORD="+pass)
	}

	if len(pathDirs) > 0 {
		out = append(out, "PATH="+joinPATH(pathDirs))
	}
	return out
}

// MavenCommand injects -s settings.xml when the env has one.
func MavenCommand(command, settingsXML string) string {
	command = strings.TrimSpace(command)
	if command == "" {
		command = "mvn -B -DskipTests package"
	}
	settingsXML = strings.TrimSpace(settingsXML)
	if settingsXML == "" {
		return command
	}
	if strings.Contains(command, " -s ") || strings.Contains(command, "--settings") {
		return command
	}
	flag := "-s " + quoteCmd(settingsXML)
	if strings.HasPrefix(command, "mvn ") {
		return "mvn " + flag + " " + strings.TrimPrefix(command, "mvn ")
	}
	if command == "mvn" {
		return "mvn " + flag
	}
	return command
}

func binDir(p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return ""
	}
	st, err := os.Stat(p)
	if err != nil {
		return filepath.Dir(p)
	}
	if st.IsDir() {
		return p
	}
	return filepath.Dir(p)
}

func joinPATH(dirs []string) string {
	sep := string(os.PathListSeparator)
	seen := map[string]bool{}
	var parts []string
	for _, d := range dirs {
		d = strings.TrimSpace(d)
		if d == "" || seen[d] {
			continue
		}
		seen[d] = true
		parts = append(parts, d)
	}
	cur := os.Getenv("PATH")
	if cur != "" {
		parts = append(parts, cur)
	}
	return strings.Join(parts, sep)
}

func mapString(m map[string]any, key string) string {
	if m == nil {
		return ""
	}
	return asString(m[key])
}

func quoteCmd(p string) string {
	if runtime.GOOS == "windows" {
		return `"` + strings.ReplaceAll(p, `"`, `\"`) + `"`
	}
	if strings.ContainsAny(p, " \t") {
		return "'" + strings.ReplaceAll(p, `'`, `'\''`) + "'"
	}
	return p
}
