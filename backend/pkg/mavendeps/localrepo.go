package mavendeps

import (
	"os"
	"path/filepath"
	"strings"
)

// DefaultLocalRepository is ResolveLocalRepository without MAVEN_HOME.
func DefaultLocalRepository(settingsXML string) string {
	return ResolveLocalRepository(settingsXML, "")
}

// ResolveLocalRepository matches Maven's local repository resolution order.
func ResolveLocalRepository(settingsXML, mavenHome string) string {
	settingsXML = strings.TrimSpace(settingsXML)
	mavenHome = resolveMavenHome(mavenHome)
	fallback := mavenHomeLocalRepo(mavenHome)

	if repo := localRepositoryFromSettings(settingsXML); repo != "" {
		normalized := normalizeLocalRepoPath(repo, settingsXML, mavenHome)
		if !isPlaceholderLocalRepository(normalized) {
			return normalized
		}
	}
	if fallback != "" {
		return fallback
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ""
	}
	return filepath.Join(home, ".m2", "repository")
}

func resolveMavenHome(mavenHome string) string {
	mavenHome = strings.TrimSpace(os.ExpandEnv(mavenHome))
	if mavenHome == "" {
		mavenHome = strings.TrimSpace(os.Getenv("MAVEN_HOME"))
	}
	return mavenHome
}

func mavenHomeLocalRepo(mavenHome string) string {
	if mavenHome == "" {
		return ""
	}
	return filepath.Clean(filepath.Join(mavenHome, "localRepository"))
}

// RawLocalRepositoryFromSettings returns the raw localRepository value from settings.xml.
func RawLocalRepositoryFromSettings(settingsXML string) string {
	return localRepositoryFromSettings(settingsXML)
}

// IsPlaceholderLocalRepositoryPath reports template paths that must not be used.
func IsPlaceholderLocalRepositoryPath(path string) bool {
	return isPlaceholderLocalRepository(path)
}

func isPlaceholderLocalRepository(path string) bool {
	p := strings.ToLower(filepath.ToSlash(strings.TrimSpace(path)))
	if p == "" {
		return true
	}
	if strings.Contains(p, "path/to/local") {
		return true
	}
	if strings.Contains(p, "/path/to/") {
		return true
	}
	if strings.Contains(p, "your/local/repo") {
		return true
	}
	return false
}

func normalizeLocalRepoPath(repo, settingsXML, mavenHome string) string {
	repo = strings.TrimSpace(os.ExpandEnv(repo))
	if repo == "" {
		return ""
	}
	if strings.HasPrefix(repo, "~") {
		if home, err := os.UserHomeDir(); err == nil && home != "" {
			repo = filepath.Join(home, strings.TrimPrefix(repo, "~"))
		}
	}
	if !filepath.IsAbs(repo) {
		base := filepath.Dir(settingsXML)
		if base == "" || base == "." {
			mavenHome = strings.TrimSpace(os.ExpandEnv(mavenHome))
			if mavenHome == "" {
				mavenHome = strings.TrimSpace(os.Getenv("MAVEN_HOME"))
			}
			if mavenHome != "" {
				base = mavenHome
			}
		}
		repo = filepath.Join(base, repo)
	}
	return filepath.Clean(repo)
}

func localRepositoryFromSettings(settingsXML string) string {
	settingsXML = strings.TrimSpace(settingsXML)
	if settingsXML == "" {
		return ""
	}
	raw, err := os.ReadFile(settingsXML)
	if err != nil {
		return ""
	}
	const open = "<localRepository>"
	const close = "</localRepository>"
	s := string(raw)
	i := strings.Index(s, open)
	if i < 0 {
		return ""
	}
	s = s[i+len(open):]
	j := strings.Index(s, close)
	if j < 0 {
		return ""
	}
	return strings.TrimSpace(s[:j])
}

func artifactJarPath(localRepo string, coord Coordinates) string {
	segments := strings.Split(coord.GroupID, ".")
	segments = append(segments, coord.ArtifactID, coord.Version, coord.ArtifactID+"-"+coord.Version+".jar")
	return filepath.Join(append([]string{localRepo}, segments...)...)
}

// ArtifactInstalled reports whether the main jar exists in the local Maven repository.
func ArtifactInstalled(localRepo string, coord Coordinates) bool {
	for _, repo := range localRepositoryCandidates(localRepo, "") {
		if artifactInstalledAt(repo, coord) {
			return true
		}
	}
	return false
}

func localRepositoryCandidates(primary, mavenHome string) []string {
	seen := map[string]struct{}{}
	var out []string
	add := func(p string) {
		p = strings.TrimSpace(p)
		if p == "" || isPlaceholderLocalRepository(p) {
			return
		}
		if _, ok := seen[p]; ok {
			return
		}
		seen[p] = struct{}{}
		out = append(out, p)
	}
	add(primary)
	add(mavenHomeLocalRepo(resolveMavenHome(mavenHome)))
	return out
}

func artifactInstalledAt(localRepo string, coord Coordinates) bool {
	localRepo = strings.TrimSpace(localRepo)
	if localRepo == "" || coord.GroupID == "" || coord.ArtifactID == "" || coord.Version == "" {
		return false
	}
	if jarInstalledAt(localRepo, coord) {
		return true
	}
	for _, altGroup := range fluxGroupIDAlternates(coord.GroupID) {
		alt := coord
		alt.GroupID = altGroup
		if jarInstalledAt(localRepo, alt) {
			return true
		}
	}
	return false
}

func jarInstalledAt(localRepo string, coord Coordinates) bool {
	path := artifactJarPath(localRepo, coord)
	info, err := os.Stat(path)
	if err != nil || info.IsDir() || info.Size() == 0 {
		return false
	}
	return true
}
