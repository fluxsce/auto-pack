// Package slug 校验会进磁盘目录的名称与相对路径，供 workspace / project 共用。
package slug

import (
	"errors"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

var (
	// ErrInvalidSlug 会进磁盘目录的名称：字母数字、点、下划线、连字符（如 V9.0.0-P09）。
	ErrInvalidSlug = errors.New("must be letters, digits, dot, underscore or hyphen")
	// ErrInvalidCode 与 ErrInvalidSlug 相同，保留给空间编码调用方。
	ErrInvalidCode = ErrInvalidSlug
	// ErrInvalidName 显示名称不能包含路径分隔或通配符。
	ErrInvalidName = errors.New("name cannot contain path characters")
	slugRe         = regexp.MustCompile(`^[A-Za-z0-9](?:[A-Za-z0-9._-]{0,62}[A-Za-z0-9])?$`)
	unsafeNameRe   = regexp.MustCompile(`[\x00-\x1f\\/:*?"<>|]`)
	reservedNames  = map[string]struct{}{
		"CON": {}, "PRN": {}, "AUX": {}, "NUL": {},
		"COM1": {}, "COM2": {}, "COM3": {}, "COM4": {}, "COM5": {},
		"COM6": {}, "COM7": {}, "COM8": {}, "COM9": {},
		"LPT1": {}, "LPT2": {}, "LPT3": {}, "LPT4": {}, "LPT5": {},
		"LPT6": {}, "LPT7": {}, "LPT8": {}, "LPT9": {},
	}
)

// CheckSlug 校验会进磁盘目录的名称。允许点号，便于 V9.0.0-P09 当工程名或编码。
func CheckSlug(name string) error {
	name = strings.TrimSpace(name)
	if !slugRe.MatchString(name) || strings.Contains(name, "..") {
		return ErrInvalidSlug
	}
	if _, bad := reservedNames[strings.ToUpper(name)]; bad {
		return ErrInvalidSlug
	}
	return nil
}

// CheckCode 校验空间编码，用作目录名。规则与 CheckSlug 相同。
func CheckCode(code string) error {
	return CheckSlug(code)
}

// CheckDisplayName 校验显示名称：必填，禁止路径分隔符与通配符。
func CheckDisplayName(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return errors.New("name required")
	}
	if unsafeNameRe.MatchString(name) || utf8.RuneCountInString(name) > 80 {
		return ErrInvalidName
	}
	for _, r := range name {
		if unicode.IsControl(r) {
			return ErrInvalidName
		}
	}
	return nil
}

// CheckRelPath 校验工程相对目录；空则跳过。每一段都必须是合法 slug。
func CheckRelPath(rel string) error {
	rel = strings.TrimSpace(rel)
	if rel == "" {
		return nil
	}
	rel = filepath.ToSlash(rel)
	if filepath.IsAbs(rel) || strings.Contains(rel, "..") {
		return ErrInvalidSlug
	}
	for _, part := range strings.Split(rel, "/") {
		if part == "" {
			continue
		}
		if err := CheckSlug(part); err != nil {
			return err
		}
	}
	return nil
}

// Name 把空间编码收成安全目录名；非法时退回 recID。
func Name(code, recID string) string {
	code = strings.TrimSpace(code)
	if CheckCode(code) == nil {
		return code
	}
	if recID != "" {
		return recID
	}
	return "workspace"
}
