package utils

import (
	"fmt"
	"path"
	"strings"
	"time"
)

// UpdateNoteName 是远端推送目录里的说明文件名。
const UpdateNoteName = "updateNote.txt"

// RemoteDir 把路径收成 POSIX 目录：以 / 开头、无尾斜杠；空则为 /。
func RemoteDir(p string) string {
	s := strings.TrimSpace(strings.ReplaceAll(p, "\\", "/"))
	s = strings.TrimRight(s, "/")
	if s == "" {
		return "/"
	}
	if !strings.HasPrefix(s, "/") {
		s = "/" + s
	}
	return s
}

// RemoteJoin 拼接远端目录与文件名。
func RemoteJoin(dir, name string) string {
	dir = RemoteDir(dir)
	name = strings.Trim(strings.ReplaceAll(strings.TrimSpace(name), "\\", "/"), "/")
	if name == "" {
		return dir
	}
	if dir == "/" {
		return "/" + name
	}
	return dir + "/" + name
}

// PushLayout 解释推送路径：末段等于文件名时视为完整文件路径，否则视为目录。
func PushLayout(pushPath, fileName string) (pushDir, updatePath string) {
	trimmed := strings.TrimSpace(strings.ReplaceAll(pushPath, "\\", "/"))
	trimmed = strings.TrimRight(trimmed, "/")
	if trimmed == "" {
		trimmed = "/"
	}
	if !strings.HasPrefix(trimmed, "/") {
		trimmed = "/" + trimmed
	}
	fileName = strings.TrimSpace(fileName)
	if fileName != "" && path.Base(trimmed) == fileName {
		dir := path.Dir(trimmed)
		if dir == "." || dir == "" {
			dir = "/"
		}
		return RemoteDir(dir), trimmed
	}
	dir := RemoteDir(pushPath)
	return dir, RemoteJoin(dir, fileName)
}

// BackupDirStamp 是 BAK 子目录名：YYYYMMDDHHmmssSSS。
func BackupDirStamp(ts time.Time) string {
	if ts.IsZero() {
		ts = time.Now()
	}
	return ts.Format("20060102150405") + fmt.Sprintf("%03d", ts.Nanosecond()/1e6)
}

// DefaultBackupDir 是推送目录下 BAK/{时间戳}。
func DefaultBackupDir(pushDir string, ts time.Time) string {
	return RemoteJoin(RemoteDir(pushDir), "BAK/"+BackupDirStamp(ts))
}
