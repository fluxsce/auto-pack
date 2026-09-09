package utils

import (
	"io/fs"
	"path/filepath"
	"strings"
)

const (
	defaultFindLimit = 80
	defaultFindDepth = 8
)

// FindRel 在 root 下按文件名查找相对路径。跳过构建产物与隐藏目录。
func FindRel(root string, names []string, limit int) ([]string, error) {
	if _, _, err := Resolve(root, ""); err != nil {
		return nil, err
	}
	want := map[string]bool{}
	for _, name := range names {
		name = strings.ToLower(strings.TrimSpace(name))
		if name != "" {
			want[name] = true
		}
	}
	if len(want) == 0 {
		return nil, nil
	}
	if limit <= 0 {
		limit = defaultFindLimit
	}
	var out []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if d.IsDir() {
			if path == root {
				return nil
			}
			if skipWalkDir(d.Name()) || strings.Count(rel, "/") >= defaultFindDepth {
				return fs.SkipDir
			}
			return nil
		}
		if !want[strings.ToLower(d.Name())] {
			return nil
		}
		cleaned, err := CleanRel(rel)
		if err != nil {
			return nil
		}
		out = append(out, cleaned)
		if len(out) >= limit {
			return fs.SkipAll
		}
		return nil
	})
	return out, err
}

func skipWalkDir(name string) bool {
	if name == "" || strings.HasPrefix(name, ".") {
		return true
	}
	switch strings.ToLower(name) {
	case "target", "node_modules", "dist", "vendor", "bin", "obj", "__pycache__":
		return true
	default:
		return false
	}
}
