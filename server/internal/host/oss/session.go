package oss

import (
	"fmt"
	"io"
	"strings"
)

// Session 把 OSS 桶当作远程推送目录：对象键去掉前导斜杠，备份用服务端复制。
type Session struct {
	cfg Config
}

// Open 校验连接信息并返回可上传的会话。
func Open(cfg Config) (*Session, error) {
	if _, _, err := endpointHost(cfg); err != nil {
		return nil, err
	}
	return &Session{cfg: cfg}, nil
}

func (s *Session) Close() error { return nil }

func (s *Session) Exists(remote string) (bool, error) {
	return Exists(s.cfg, objectKey(remote))
}

func (s *Session) Download(remote string) ([]byte, error) {
	return Get(s.cfg, objectKey(remote))
}

func (s *Session) UploadFile(local, remote string, w io.Writer) error {
	key := objectKey(remote)
	if w != nil {
		_, _ = fmt.Fprintf(w, "OSS PUT %s\n", key)
	}
	return PutFile(s.cfg, local, key)
}

func (s *Session) UploadBytes(remote string, data []byte, w io.Writer) error {
	key := objectKey(remote)
	if w != nil {
		_, _ = fmt.Fprintf(w, "OSS PUT %s\n", key)
	}
	return PutBytes(s.cfg, key, data)
}

func (s *Session) Backup(src, dest string, w io.Writer) error {
	from, to := objectKey(src), objectKey(dest)
	if w != nil {
		_, _ = fmt.Fprintf(w, "OSS COPY %s → %s\n", from, to)
	}
	return Copy(s.cfg, from, to)
}

func objectKey(remote string) string {
	return strings.TrimLeft(strings.ReplaceAll(strings.TrimSpace(remote), "\\", "/"), "/")
}
