package ftpclient

import (
	"fmt"
	"io"
	"os"
	"path"
	"strings"
	"time"

	"auto-pack/backend/pkg/envtypes"

	"github.com/jlaffaye/ftp"
)

// OpenSession dials and logs in to the FTP server.
func OpenSession(cfg envtypes.FTPConfig) (*ftp.ServerConn, error) {
	cfg.Port = envtypes.DefaultPort(cfg.Port, 21)
	addr := fmt.Sprintf("%s:%d", strings.TrimSpace(cfg.Host), cfg.Port)
	timeout := time.Duration(envtypes.DefaultTimeoutSec(cfg.TimeoutSec)) * time.Second

	conn, err := ftp.Dial(addr, ftp.DialWithTimeout(timeout))
	if err != nil {
		return nil, fmt.Errorf("ftp dial: %w", err)
	}
	if err := conn.Login(strings.TrimSpace(cfg.Username), cfg.Password); err != nil {
		_ = conn.Quit()
		return nil, fmt.Errorf("ftp login: %w", err)
	}
	return conn, nil
}

// RemoteExists reports whether a remote path exists under base_path.
func RemoteExists(conn *ftp.ServerConn, cfg envtypes.FTPConfig, rel string) (bool, error) {
	remote := joinFTPPath(normalizeBase(cfg.BasePath), rel)
	dir := path.Dir(remote)
	name := path.Base(remote)
	if dir == "." {
		dir = "/"
	}
	entries, err := conn.List(dir)
	if err != nil {
		if isNotExistListErr(err) {
			return false, nil
		}
		return false, err
	}
	for _, e := range entries {
		if e.Name == name {
			return true, nil
		}
	}
	return false, nil
}

func isNotExistListErr(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "550") || strings.Contains(msg, "not found")
}

// RenameRemote moves a remote file (RNFR/RNTO).
func RenameRemote(conn *ftp.ServerConn, cfg envtypes.FTPConfig, srcRel, dstRel string) error {
	base := normalizeBase(cfg.BasePath)
	src := joinFTPPath(base, srcRel)
	dst := joinFTPPath(base, dstRel)
	if err := mkdirFTP(conn, path.Dir(dst)); err != nil {
		return err
	}
	if err := conn.Rename(src, dst); err != nil {
		return fmt.Errorf("rename %s -> %s: %w", src, dst, err)
	}
	return nil
}

// BackupRemoteFile moves the remote file to dst when possible; otherwise copies (source kept).
// If dst already exists the backup is skipped (no overwrite, no delete).
func BackupRemoteFile(conn *ftp.ServerConn, cfg envtypes.FTPConfig, srcRel, dstRel string) error {
	dstExists, err := RemoteExists(conn, cfg, dstRel)
	if err != nil {
		return err
	}
	if dstExists {
		return nil
	}
	if err := RenameRemote(conn, cfg, srcRel, dstRel); err == nil {
		return nil
	}
	return CopyRemote(conn, cfg, srcRel, dstRel)
}

// DownloadRemote reads the full content of a remote file relative to base_path.
func DownloadRemote(conn *ftp.ServerConn, cfg envtypes.FTPConfig, rel string) ([]byte, error) {
	base := normalizeBase(cfg.BasePath)
	remote := joinFTPPath(base, rel)
	resp, err := conn.Retr(remote)
	if err != nil {
		return nil, fmt.Errorf("retr %s: %w", remote, err)
	}
	defer resp.Close()
	data, err := io.ReadAll(resp)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", remote, err)
	}
	return data, nil
}

// CopyRemote copies a remote file to another remote path (download + upload).
// If dst already exists the copy is skipped.
func CopyRemote(conn *ftp.ServerConn, cfg envtypes.FTPConfig, srcRel, dstRel string) error {
	dstExists, err := RemoteExists(conn, cfg, dstRel)
	if err != nil {
		return err
	}
	if dstExists {
		return nil
	}
	base := normalizeBase(cfg.BasePath)
	src := joinFTPPath(base, srcRel)
	dst := joinFTPPath(base, dstRel)
	if err := mkdirFTP(conn, path.Dir(dst)); err != nil {
		return err
	}
	resp, err := conn.Retr(src)
	if err != nil {
		return fmt.Errorf("retr %s: %w", src, err)
	}
	defer resp.Close()
	data, err := io.ReadAll(resp)
	if err != nil {
		return fmt.Errorf("read %s: %w", src, err)
	}
	return UploadBytes(conn, cfg, dstRel, data)
}

// UploadBytes stores content at a remote path relative to base_path.
func UploadBytes(conn *ftp.ServerConn, cfg envtypes.FTPConfig, rel string, content []byte) error {
	base := normalizeBase(cfg.BasePath)
	remote := joinFTPPath(base, rel)
	if err := mkdirFTP(conn, path.Dir(remote)); err != nil {
		return fmt.Errorf("ensure remote dir: %w", err)
	}
	if err := storInDir(conn, remote, strings.NewReader(string(content))); err != nil {
		return fmt.Errorf("stor %s: %w", remote, err)
	}
	return nil
}

func normalizeBase(base string) string {
	base = strings.TrimSpace(base)
	if base == "" {
		return "/"
	}
	return path.Clean(strings.ReplaceAll(base, "\\", "/"))
}

// UploadLocalFile stores a local file at rel relative to base_path using an open session.
func UploadLocalFile(conn *ftp.ServerConn, cfg envtypes.FTPConfig, rel, localPath string) error {
	f, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("open %s: %w", localPath, err)
	}
	defer f.Close()
	return UploadReader(conn, cfg, rel, f)
}

// UploadReader stores from r at a remote path relative to base_path.
func UploadReader(conn *ftp.ServerConn, cfg envtypes.FTPConfig, rel string, r io.Reader) error {
	base := normalizeBase(cfg.BasePath)
	remote := joinFTPPath(base, rel)
	if err := mkdirFTP(conn, path.Dir(remote)); err != nil {
		return fmt.Errorf("ensure remote dir: %w", err)
	}
	if err := storInDir(conn, remote, r); err != nil {
		return fmt.Errorf("stor %s: %w", remote, err)
	}
	return nil
}
