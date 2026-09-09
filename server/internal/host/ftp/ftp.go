// Package ftp 上传文件。无 HTTP、无权限表。
package ftp

import (
	"io"
	"path"
	"path/filepath"
	"strconv"
	"strings"
)

// Config is an FTP endpoint.
type Config struct {
	Host      string
	Port      string
	Username  string
	Password  string
	RemoteDir string
}

// UploadFile 一次性拨号，把 localPath 存成 RemoteDir/remoteName。
func UploadFile(cfg Config, localPath, remoteName string, w io.Writer) error {
	s, err := Dial(cfg)
	if err != nil {
		return err
	}
	defer s.Close()
	if remoteName == "" {
		remoteName = filepath.Base(localPath)
	}
	remote := remoteName
	if dir := strings.TrimSpace(cfg.RemoteDir); dir != "" {
		remote = path.Join(dir, remoteName)
	}
	return s.UploadFile(localPath, remote, w)
}

// MapConfig reads a decrypted env config map.
func MapConfig(m map[string]any) Config {
	str := func(keys ...string) string {
		if m == nil {
			return ""
		}
		for _, k := range keys {
			if v, ok := m[k]; ok {
				if s := anyString(v); s != "" {
					return s
				}
			}
		}
		return ""
	}
	return Config{
		Host:      str("host"),
		Port:      str("port"),
		Username:  str("username"),
		Password:  str("password"),
		RemoteDir: str("remote_dir", "base_path", "remote_path"),
	}
}

func anyString(v any) string {
	switch t := v.(type) {
	case string:
		return strings.TrimSpace(t)
	case float64:
		if t == float64(int64(t)) {
			return strconv.FormatInt(int64(t), 10)
		}
		return strconv.FormatFloat(t, 'f', -1, 64)
	case int:
		return strconv.Itoa(t)
	case int64:
		return strconv.FormatInt(t, 10)
	default:
		return ""
	}
}
