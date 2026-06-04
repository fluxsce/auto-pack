package sftpclient

import (
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	"auto-pack/backend/pkg/envtypes"
	"auto-pack/backend/pkg/sshclient"

	"github.com/pkg/sftp"
)

// UploadFile uploads a single local file to remotePath (absolute or relative to remote base).
func UploadFile(ctx context.Context, cfg envtypes.SFTPConfig, localPath, remotePath string) error {
	return UploadFiles(ctx, cfg, []UploadItem{{LocalPath: localPath, RemotePath: remotePath}})
}

// UploadItem is one file to upload.
type UploadItem struct {
	LocalPath  string
	RemotePath string
}

// UploadFiles uploads files over SFTP (SSH).
func UploadFiles(ctx context.Context, cfg envtypes.SFTPConfig, items []UploadItem) error {
	if len(items) == 0 {
		return nil
	}
	sshCfg := envtypes.SSHConfig{
		Host:           cfg.Host,
		Port:           cfg.Port,
		Username:       cfg.Username,
		Password:       cfg.Password,
		PrivateKeyPath: cfg.PrivateKeyPath,
		TimeoutSec:     cfg.TimeoutSec,
	}
	client, err := sshclient.Dial(ctx, sshCfg)
	if err != nil {
		return err
	}
	defer client.Close()

	sftpClient, err := sftp.NewClient(client)
	if err != nil {
		return fmt.Errorf("sftp client: %w", err)
	}
	defer sftpClient.Close()

	for _, item := range items {
		if err := uploadOne(sftpClient, item.LocalPath, item.RemotePath); err != nil {
			return err
		}
	}
	return nil
}

func uploadOne(client *sftp.Client, localPath, remotePath string) error {
	localPath = filepath.Clean(localPath)
	remotePath = path.Clean(strings.ReplaceAll(remotePath, "\\", "/"))
	if remotePath == "." {
		return fmt.Errorf("invalid remote path for %s", localPath)
	}

	if err := mkdirAll(client, path.Dir(remotePath)); err != nil {
		return fmt.Errorf("mkdir remote %s: %w", path.Dir(remotePath), err)
	}

	src, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("open %s: %w", localPath, err)
	}
	defer src.Close()

	dst, err := client.Create(remotePath)
	if err != nil {
		return fmt.Errorf("create remote %s: %w", remotePath, err)
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return fmt.Errorf("upload %s: %w", localPath, err)
	}
	return nil
}

func mkdirAll(client *sftp.Client, dir string) error {
	dir = path.Clean(dir)
	if dir == "." || dir == "/" {
		return nil
	}
	if err := client.MkdirAll(dir); err != nil {
		// MkdirAll may fail if exists; try stat
		if _, statErr := client.Stat(dir); statErr == nil {
			return nil
		}
		return err
	}
	return nil
}
