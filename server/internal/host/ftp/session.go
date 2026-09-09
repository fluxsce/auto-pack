package ftp

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path"
	"strings"

	"github.com/jlaffaye/ftp"
)

// Session 是一次 FTP 登录，供推送时复用连接做备份与写说明。
type Session struct {
	conn *ftp.ServerConn
}

// Dial 拨号并登录。调用方必须 Close。
func Dial(cfg Config) (*Session, error) {
	addr := cfg.Host
	if cfg.Port != "" {
		addr = cfg.Host + ":" + cfg.Port
	} else if !strings.Contains(addr, ":") {
		addr = addr + ":21"
	}
	c, err := ftp.Dial(addr)
	if err != nil {
		return nil, err
	}
	if err := c.Login(cfg.Username, cfg.Password); err != nil {
		_ = c.Quit()
		return nil, err
	}
	return &Session{conn: c}, nil
}

// Close 断开会话。
func (s *Session) Close() error {
	if s == nil || s.conn == nil {
		return nil
	}
	return s.conn.Quit()
}

// Exists 报告远端文件是否存在。
func (s *Session) Exists(remote string) (bool, error) {
	remote = posix(remote)
	if remote == "" || remote == "/" {
		return false, nil
	}
	if _, err := s.conn.FileSize(remote); err == nil {
		return true, nil
	}
	r, err := s.conn.Retr(remote)
	if err != nil {
		return false, nil
	}
	_ = r.Close()
	return true, nil
}

// Download 读取远端文件。
func (s *Session) Download(remote string) ([]byte, error) {
	remote = posix(remote)
	r, err := s.conn.Retr(remote)
	if err != nil {
		return nil, fmt.Errorf("ftp retr %s: %w", remote, err)
	}
	defer r.Close()
	return io.ReadAll(r)
}

// UploadFile 把本地文件存到远端完整路径，必要时创建父目录。
func (s *Session) UploadFile(local, remote string, w io.Writer) error {
	f, err := os.Open(local)
	if err != nil {
		return err
	}
	defer f.Close()
	return s.upload(remote, f, w)
}

// UploadBytes 把内存内容存到远端完整路径。
func (s *Session) UploadBytes(remote string, data []byte, w io.Writer) error {
	return s.upload(remote, bytes.NewReader(data), w)
}

// Backup 把已有远端文件拷到 dest（RETR + STOR），覆盖前保留一份。
func (s *Session) Backup(src, dest string, w io.Writer) error {
	data, err := s.Download(src)
	if err != nil {
		return err
	}
	if w != nil {
		_, _ = fmt.Fprintf(w, "备份 %s → %s\n", posix(src), posix(dest))
	}
	return s.UploadBytes(dest, data, nil)
}

func (s *Session) upload(remote string, r io.Reader, w io.Writer) error {
	remote = posix(remote)
	dir := path.Dir(remote)
	name := path.Base(remote)
	if name == "" || name == "." || name == "/" {
		return fmt.Errorf("invalid remote file name for %s", remote)
	}
	if err := s.mkdirAll(dir); err != nil {
		return err
	}
	if err := s.conn.ChangeDir(dir); err != nil {
		return fmt.Errorf("ftp cwd %s: %w", dir, err)
	}
	if w != nil {
		_, _ = fmt.Fprintf(w, "STOR %s\n", remote)
	}
	return s.conn.Stor(name, r)
}

func (s *Session) mkdirAll(dir string) error {
	dir = posix(dir)
	if dir == "." || dir == "/" || dir == "" {
		return nil
	}
	if err := s.conn.ChangeDir(dir); err == nil {
		return nil
	}
	parts := strings.Split(strings.Trim(dir, "/"), "/")
	cur := ""
	for _, p := range parts {
		if p == "" || p == "." {
			continue
		}
		if cur == "" {
			cur = "/" + p
		} else {
			cur = cur + "/" + p
		}
		if err := s.conn.ChangeDir(cur); err == nil {
			continue
		}
		parent := path.Dir(cur)
		if parent == "" || parent == "." {
			parent = "/"
		}
		if err := s.conn.ChangeDir(parent); err != nil {
			return fmt.Errorf("ftp cwd %s: %w", parent, err)
		}
		if err := s.conn.MakeDir(p); err != nil {
			if err2 := s.conn.ChangeDir(cur); err2 == nil {
				continue
			}
			return fmt.Errorf("ftp mkdir %s: %w", cur, err)
		}
	}
	return nil
}

// ReadRemote 一次性拨号读取远端文件；不存在时 exists=false。
func ReadRemote(cfg Config, remote string) (exists bool, data []byte, err error) {
	s, err := Dial(cfg)
	if err != nil {
		return false, nil, err
	}
	defer s.Close()
	ok, err := s.Exists(remote)
	if err != nil || !ok {
		return false, nil, err
	}
	raw, err := s.Download(remote)
	if err != nil {
		return false, nil, err
	}
	return true, raw, nil
}

func posix(p string) string {
	p = strings.TrimSpace(strings.ReplaceAll(p, "\\", "/"))
	if p == "" {
		return "/"
	}
	return path.Clean(p)
}
