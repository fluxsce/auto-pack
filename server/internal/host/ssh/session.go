package ssh

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/pkg/sftp"
	cryptossh "golang.org/x/crypto/ssh"
)

// Session 是一次 SSH+SFTP 连接，供推送时复用做备份与写说明。
type Session struct {
	client *cryptossh.Client
	sftp   *sftp.Client
}

// DialSession 拨号并打开 SFTP。调用方必须 Close。
func DialSession(cfg Config) (*Session, error) {
	client, err := Dial(cfg)
	if err != nil {
		return nil, err
	}
	sc, err := sftp.NewClient(client)
	if err != nil {
		_ = client.Close()
		return nil, err
	}
	return &Session{client: client, sftp: sc}, nil
}

// Close 关闭 SFTP 与 SSH。
func (s *Session) Close() error {
	if s == nil {
		return nil
	}
	if s.sftp != nil {
		_ = s.sftp.Close()
	}
	if s.client != nil {
		return s.client.Close()
	}
	return nil
}

// Exists 报告远端文件是否存在。
func (s *Session) Exists(remote string) (bool, error) {
	remote = posix(remote)
	_, err := s.sftp.Stat(remote)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

// Download 读取远端文件。
func (s *Session) Download(remote string) ([]byte, error) {
	remote = posix(remote)
	f, err := s.sftp.Open(remote)
	if err != nil {
		return nil, fmt.Errorf("sftp open %s: %w", remote, err)
	}
	defer f.Close()
	return io.ReadAll(f)
}

// UploadFile 把本地文件存到远端完整路径。
func (s *Session) UploadFile(local, remote string, w io.Writer) error {
	src, err := os.Open(local)
	if err != nil {
		return err
	}
	defer src.Close()
	return s.upload(remote, src, w)
}

// UploadBytes 把内存内容存到远端完整路径。
func (s *Session) UploadBytes(remote string, data []byte, w io.Writer) error {
	return s.upload(remote, bytes.NewReader(data), w)
}

// Backup 把已有远端文件重命名到 dest，不传输文件内容。
func (s *Session) Backup(src, dest string, w io.Writer) error {
	src = posix(src)
	dest = posix(dest)
	dir := path.Dir(dest)
	if dir != "" && dir != "." && dir != "/" {
		if err := s.sftp.MkdirAll(dir); err != nil {
			return fmt.Errorf("sftp mkdir %s: %w", dir, err)
		}
	}
	if w != nil {
		_, _ = fmt.Fprintf(w, "备份 %s → %s\n", src, dest)
	}
	if err := s.sftp.Rename(src, dest); err != nil {
		return fmt.Errorf("sftp rename %s: %w", src, err)
	}
	return nil
}

func (s *Session) upload(remote string, r io.Reader, w io.Writer) error {
	remote = posix(remote)
	dir := path.Dir(remote)
	if dir != "" && dir != "." && dir != "/" {
		if err := s.sftp.MkdirAll(dir); err != nil {
			return fmt.Errorf("sftp mkdir %s: %w", dir, err)
		}
	}
	dst, err := s.sftp.Create(remote)
	if err != nil {
		return fmt.Errorf("sftp create %s: %w", remote, err)
	}
	defer dst.Close()
	if w != nil {
		_, _ = fmt.Fprintf(w, "SFTP %s\n", remote)
	}
	_, err = io.Copy(dst, r)
	return err
}

// ReadRemote 一次性拨号读取远端文件；不存在时 exists=false。
func ReadRemote(cfg Config, remote string) (exists bool, data []byte, err error) {
	s, err := DialSession(cfg)
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

// UploadFile 一次性拨号，把 localPath 存成 RemotePath/remoteName。
func UploadFile(cfg Config, localPath, remoteName string, w io.Writer) error {
	s, err := DialSession(cfg)
	if err != nil {
		return err
	}
	defer s.Close()
	if remoteName == "" {
		remoteName = filepath.Base(localPath)
	}
	remote := remoteName
	if dir := strings.TrimSpace(cfg.RemotePath); dir != "" && dir != "." {
		remote = path.Join(dir, remoteName)
	}
	return s.UploadFile(localPath, remote, w)
}
