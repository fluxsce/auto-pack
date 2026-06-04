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

// BackupRemoteFile moves the remote file to dst when possible; otherwise copies (source kept until overwrite).
func BackupRemoteFile(conn *ftp.ServerConn, cfg envtypes.FTPConfig, srcRel, dstRel string) error {
	if err := RenameRemote(conn, cfg, srcRel, dstRel); err == nil {
		return nil
	}
	return CopyRemote(conn, cfg, srcRel, dstRel)
}

// CopyRemote copies a remote file to another remote path (download + upload).
func CopyRemote(conn *ftp.ServerConn, cfg envtypes.FTPConfig, srcRel, dstRel string) error {
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
	if err := conn.Stor(dst, resp); err != nil {
		return fmt.Errorf("stor %s: %w", dst, err)
	}
	return nil
}

// UploadBytes stores content at a remote path relative to base_path.
func UploadBytes(conn *ftp.ServerConn, cfg envtypes.FTPConfig, rel string, content []byte) error {
	base := normalizeBase(cfg.BasePath)
	remote := joinFTPPath(base, rel)
	if err := mkdirFTP(conn, path.Dir(remote)); err != nil {
		return err
	}
	return conn.Stor(remote, strings.NewReader(string(content)))
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
		return err
	}
	return conn.Stor(remote, r)
}
