package utils

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

var (
	// ErrOutside 请求路径跳出了给定根目录。
	ErrOutside = errors.New("path outside root")
	// ErrNotFile 目标不是普通文件。
	ErrNotFile = errors.New("not a file")
	// ErrNotDir 目标不是目录。
	ErrNotDir = errors.New("not a directory")
	// ErrNotText 目标不是 UTF-8 文本。
	ErrNotText = errors.New("not a text file")
)

// CleanRel 收成相对根的斜杠路径；空与 "." 视为根。禁止绝对路径与 ..。
func CleanRel(rel string) (string, error) {
	rel = strings.TrimSpace(filepath.ToSlash(rel))
	rel = strings.TrimPrefix(rel, "/")
	if rel == "" || rel == "." {
		return "", nil
	}
	if filepath.IsAbs(rel) || strings.Contains(rel, "..") {
		return "", ErrOutside
	}
	return rel, nil
}

// JoinRel 用斜杠拼接相对路径。
func JoinRel(dir, name string) string {
	if dir == "" {
		return name
	}
	return dir + "/" + name
}

// JoinUnder 把 rel 接到 base 下。rel 须先经过 CleanRel。
func JoinUnder(base, rel string) string {
	if rel == "" {
		return base
	}
	return filepath.Join(base, filepath.FromSlash(rel))
}

// UnderRoot 报告 dest 是否落在 root 内。
func UnderRoot(root, dest string) bool {
	root = filepath.Clean(root)
	dest = filepath.Clean(dest)
	rel, err := filepath.Rel(root, dest)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator))
}

// Resolve 在 root 下解析 rel，并确认结果仍在 root 内。
func Resolve(root, rel string) (string, string, error) {
	rel, err := CleanRel(rel)
	if err != nil {
		return "", "", err
	}
	dest := JoinUnder(root, rel)
	if !UnderRoot(root, dest) {
		return "", "", ErrOutside
	}
	return dest, rel, nil
}
