package projectcfg

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
)

// ArtifactBaseDir returns the directory used to resolve transfer source patterns.
func ArtifactBaseDir(workDir, outputDir string) string {
	workDir = strings.TrimSpace(workDir)
	outputDir = strings.TrimSpace(outputDir)
	if outputDir == "" {
		return workDir
	}
	if filepath.IsAbs(outputDir) {
		return outputDir
	}
	return filepath.Join(workDir, outputDir)
}

// MatchArtifactFiles finds files under baseDir matching pattern (supports ** globs).
func MatchArtifactFiles(baseDir, pattern string) ([]string, error) {
	baseDir = filepath.Clean(baseDir)
	if st, err := os.Stat(baseDir); err != nil {
		return nil, err
	} else if !st.IsDir() {
		return nil, nil
	}
	pattern = strings.TrimSpace(pattern)
	if pattern == "" {
		pattern = "**/*"
	}
	pattern = filepath.ToSlash(pattern)

	var matches []string
	err := filepath.WalkDir(baseDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(baseDir, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		ok, err := doublestar.Match(pattern, rel)
		if err != nil {
			return err
		}
		if ok {
			matches = append(matches, path)
		}
		return nil
	})
	return matches, err
}

// RemoteRelPath maps a local file to a remote relative path after remove_prefix.
func RemoteRelPath(localPath, baseDir, removePrefix string) string {
	rel, err := filepath.Rel(baseDir, localPath)
	if err != nil {
		rel = filepath.Base(localPath)
	}
	rel = filepath.ToSlash(rel)
	removePrefix = strings.TrimSpace(filepath.ToSlash(removePrefix))
	removePrefix = strings.Trim(removePrefix, "/")
	if removePrefix != "" {
		if rel == removePrefix {
			rel = filepath.Base(rel)
		} else if strings.HasPrefix(rel, removePrefix+"/") {
			rel = strings.TrimPrefix(rel, removePrefix+"/")
		}
	}
	return rel
}
