package ftpclient

import (
	"fmt"
	"os"
	"path"
	"strings"
	"time"

	"auto-pack/backend/pkg/envtypes"

	"github.com/jlaffaye/ftp"
)

// UploadItem is one file to upload via FTP.
type UploadItem struct {
	LocalPath  string
	RemotePath string
}

// UploadFiles stores local files on the FTP server.
func UploadFiles(cfg envtypes.FTPConfig, items []UploadItem) error {
	if len(items) == 0 {
		return nil
	}
	cfg.Port = envtypes.DefaultPort(cfg.Port, 21)
	addr := fmt.Sprintf("%s:%d", strings.TrimSpace(cfg.Host), cfg.Port)
	timeout := time.Duration(envtypes.DefaultTimeoutSec(cfg.TimeoutSec)) * time.Second

	conn, err := ftp.Dial(addr, ftp.DialWithTimeout(timeout))
	if err != nil {
		return fmt.Errorf("ftp dial: %w", err)
	}
	defer conn.Quit()

	if err := conn.Login(strings.TrimSpace(cfg.Username), cfg.Password); err != nil {
		return fmt.Errorf("ftp login: %w", err)
	}

	base := strings.TrimSpace(cfg.BasePath)
	if base == "" {
		base = "/"
	}
	base = path.Clean(strings.ReplaceAll(base, "\\", "/"))

	for _, item := range items {
		remote := joinFTPPath(base, item.RemotePath)
		if err := mkdirFTP(conn, path.Dir(remote)); err != nil {
			return err
		}
		f, err := os.Open(item.LocalPath)
		if err != nil {
			return fmt.Errorf("open %s: %w", item.LocalPath, err)
		}
		if err := conn.Stor(remote, f); err != nil {
			_ = f.Close()
			return fmt.Errorf("stor %s: %w", remote, err)
		}
		_ = f.Close()
	}
	return nil
}

func joinFTPPath(base, rel string) string {
	rel = path.Clean(strings.ReplaceAll(rel, "\\", "/"))
	if strings.HasPrefix(rel, "/") {
		return rel
	}
	if base == "/" || base == "." {
		return "/" + strings.TrimPrefix(rel, "/")
	}
	return path.Clean(base + "/" + rel)
}

func mkdirFTP(conn *ftp.ServerConn, dir string) error {
	dir = path.Clean(dir)
	if dir == "." || dir == "/" {
		return nil
	}
	parts := strings.Split(strings.Trim(dir, "/"), "/")
	cur := ""
	for _, p := range parts {
		if cur == "" {
			cur = "/" + p
		} else {
			cur = cur + "/" + p
		}
		_ = conn.MakeDir(cur)
	}
	return nil
}

// RemotePathFromRel builds an FTP path under base_path.
func RemotePathFromRel(cfg envtypes.FTPConfig, rel string) string {
	base := strings.TrimSpace(cfg.BasePath)
	if base == "" {
		base = "/"
	}
	return joinFTPPath(base, rel)
}

// LocalRelToRemote joins remote_dir with relative artifact path (POSIX).
func LocalRelToRemote(remoteDir, rel string) string {
	remoteDir = strings.TrimSpace(remoteDir)
	remoteDir = strings.ReplaceAll(remoteDir, "\\", "/")
	remoteDir = strings.TrimRight(remoteDir, "/")
	rel = strings.TrimLeft(strings.ReplaceAll(rel, "\\", "/"), "/")
	if remoteDir == "" || remoteDir == "/" {
		return "/" + rel
	}
	return remoteDir + "/" + rel
}
