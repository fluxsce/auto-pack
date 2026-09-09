package workspace

import (
	"os"
	"path/filepath"
	"strings"

	"auto-pack/server/internal/access"
	"auto-pack/server/internal/host/utils"
	"auto-pack/server/internal/slug"
)

var (
	// ErrInvalidSlug 会进磁盘目录的名称：字母数字、点、下划线、连字符。
	ErrInvalidSlug = slug.ErrInvalidSlug
	// ErrInvalidCode 与 ErrInvalidSlug 相同，保留给空间编码调用方。
	ErrInvalidCode = slug.ErrInvalidCode
	// ErrInvalidName 显示名称不能包含路径分隔或通配符。
	ErrInvalidName = slug.ErrInvalidName
)

// CheckSlug 校验会进磁盘目录的名称。允许点号，便于版本号当工程名。
func CheckSlug(name string) error {
	return slug.CheckSlug(name)
}

// CheckCode 校验空间编码，用作目录名。规则与 CheckSlug 相同。
func CheckCode(code string) error {
	return slug.CheckCode(code)
}

// CheckDisplayName 校验显示名称：必填，禁止路径分隔符与通配符。
func CheckDisplayName(name string) error {
	return slug.CheckDisplayName(name)
}

// CheckRelPath 校验工程相对目录；空则跳过。每一段都必须是合法 slug。
func CheckRelPath(rel string) error {
	return slug.CheckRelPath(rel)
}

// Slug 把空间编码收成安全目录名；非法时退回 recID。
func Slug(code, recID string) string {
	return slug.Name(code, recID)
}

// Dir 是一个工作空间在根路径下的目录：{root}/{code}/。
func Dir(root, code, recID string) string {
	return filepath.Join(root, Slug(code, recID))
}

// ProjectDir 是工程检出目录：{root}/{code}/{rel}/。rel 禁止绝对路径与 ..。
func ProjectDir(root, code, recID, rel string) string {
	rel = strings.TrimSpace(rel)
	if rel == "" {
		rel = recID
	}
	rel = filepath.ToSlash(rel)
	if filepath.IsAbs(rel) || strings.Contains(rel, "..") {
		rel = filepath.Base(rel)
	}
	return filepath.Join(Dir(root, code, recID), filepath.FromSlash(rel))
}

// UnderRoot 报告 dest 是否落在 root 内，删除磁盘目录前必须检查。
func UnderRoot(root, dest string) bool {
	return utils.UnderRoot(root, dest)
}

// EnsureRoot 创建配置的工作空间根目录，以及默认空间目录。
func (s *Service) EnsureRoot() error {
	if strings.TrimSpace(s.root) == "" {
		return nil
	}
	if err := os.MkdirAll(s.root, 0o755); err != nil {
		return err
	}
	return mkdirWorkspace(s.root, "default", access.DefaultWorkspaceRecID)
}

func mkdirWorkspace(root, code, recID string) error {
	if strings.TrimSpace(root) == "" {
		return nil
	}
	return os.MkdirAll(Dir(root, code, recID), 0o755)
}

func removeWorkspaceDir(root, code, recID string) {
	if strings.TrimSpace(root) == "" {
		return
	}
	dir := Dir(root, code, recID)
	if UnderRoot(root, dir) {
		_ = os.RemoveAll(dir)
	}
}
