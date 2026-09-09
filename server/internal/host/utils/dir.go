package utils

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const defaultListLimit = 400

// Entry 是一层目录里的文件或子目录。
type Entry struct {
	Name    string
	Path    string
	IsDir   bool
	Size    int64
	ModTime string
}

// ListDir 列举 root 下 rel 这一层。目录不存在时 exists=false 且不报错。
func ListDir(root, rel string, limit int) (exists bool, entries []Entry, err error) {
	dest, rel, err := Resolve(root, rel)
	if err != nil {
		return false, nil, err
	}
	st, err := os.Stat(dest)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil, nil
		}
		return false, nil, err
	}
	if !st.IsDir() {
		return false, nil, ErrNotDir
	}
	items, err := os.ReadDir(dest)
	if err != nil {
		return true, nil, err
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].IsDir() != items[j].IsDir() {
			return items[i].IsDir()
		}
		return strings.ToLower(items[i].Name()) < strings.ToLower(items[j].Name())
	})
	if limit <= 0 {
		limit = defaultListLimit
	}
	if len(items) > limit {
		items = items[:limit]
	}
	entries = make([]Entry, 0, len(items))
	for _, item := range items {
		name := item.Name()
		entry := Entry{Name: name, Path: JoinRel(rel, name), IsDir: item.IsDir()}
		if info, err := item.Info(); err == nil {
			if !item.IsDir() {
				entry.Size = info.Size()
			}
			entry.ModTime = info.ModTime().UTC().Format(time.RFC3339)
		}
		entries = append(entries, entry)
	}
	return true, entries, nil
}

// ClearDir 删除 root 下 rel 目录内的全部条目，保留该目录本身。
// 目录不存在时不报错。rel 须先能通过 Resolve，禁止跳出 root。
func ClearDir(root, rel string) error {
	dest, _, err := Resolve(root, rel)
	if err != nil {
		return err
	}
	st, err := os.Stat(dest)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if !st.IsDir() {
		return ErrNotDir
	}
	items, err := os.ReadDir(dest)
	if err != nil {
		return err
	}
	for _, item := range items {
		if err := os.RemoveAll(filepath.Join(dest, item.Name())); err != nil {
			return err
		}
	}
	return nil
}

// RemoveRel 删除 root 下的文件或目录。rel 为空（即 root 本身）时拒绝，避免误删工程根。
func RemoveRel(root, rel string) error {
	dest, cleaned, err := Resolve(root, rel)
	if err != nil {
		return err
	}
	if cleaned == "" {
		return ErrOutside
	}
	return os.RemoveAll(dest)
}
