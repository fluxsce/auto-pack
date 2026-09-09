package db

import (
	"path/filepath"
	"strings"
)

const sqlitePragmaQuery = "_pragma=busy_timeout(15000)" +
	"&_pragma=journal_mode(WAL)" +
	"&_pragma=synchronous(NORMAL)" +
	"&_pragma=foreign_keys(1)"

// sqliteFilePath returns the on-disk path used for mkdir (not the connection DSN).
func sqliteFilePath(raw string) string {
	s := strings.TrimSpace(raw)
	if s == "" || strings.EqualFold(s, "mem") {
		return ""
	}
	if strings.HasPrefix(s, ":memory:") || strings.Contains(s, "mode=memory") {
		return ""
	}
	if strings.HasPrefix(s, "file:") {
		// Best-effort: callers should pass a plain path; file: URIs skip auto dir creation.
		return ""
	}
	return filepath.Clean(s)
}

// sqliteOpenDSN builds a modernc.org/sqlite DSN with per-connection PRAGMAs for concurrency.
func sqliteOpenDSN(raw string) string {
	s := strings.TrimSpace(raw)
	if s == "" || strings.EqualFold(s, "mem") {
		return ":memory:"
	}
	if strings.HasPrefix(s, ":memory:") {
		return s
	}
	if strings.Contains(s, "_pragma=") {
		return s
	}
	if strings.HasPrefix(s, "file:") {
		if strings.Contains(s, "?") {
			return s
		}
		return s + "?" + sqlitePragmaQuery
	}

	p := filepath.Clean(s)
	p = filepath.ToSlash(p)
	if len(p) >= 2 && p[1] == ':' {
		return "file:///" + p + "?" + sqlitePragmaQuery
	}
	return "file:" + p + "?" + sqlitePragmaQuery
}
