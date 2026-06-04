package workspaceftppush

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"auto-pack/backend/pkg/envtypes"
	"auto-pack/backend/pkg/ftpclient"
	"auto-pack/backend/pkg/projectfs"
)

// Request is input for a workspace FTP push (environment auto-update).
type Request struct {
	LocalBase    string
	RelativePath string
	FTPConfig    envtypes.FTPConfig
	PushPath     string
	BackupPath   string
	Pusher       string
	TaskRef      string
	Detail       string
}

// Result is returned after a successful push.
type Result struct {
	BackupPath   string `json:"backup_path"`
	UpdatePath   string `json:"update_path"`
	InfoFilePath string `json:"info_file_path"`
	InfoContent  string `json:"info_content"`
}

var (
	ErrPathRequired     = errors.New("workspace path is required")
	ErrPushPathRequired = errors.New("push_path is required")
	ErrPusherRequired   = errors.New("pusher is required")
	ErrTaskRefRequired  = errors.New("task_ref is required")
	ErrNotFound         = projectfs.ErrNotFound
)

// Push uploads a workspace file or directory to FTP with backup and updateNote.txt.
func Push(req Request) (Result, error) {
	rel := strings.TrimSpace(strings.ReplaceAll(req.RelativePath, "\\", "/"))
	if rel == "" || rel == "." {
		return Result{}, ErrPathRequired
	}
	if strings.TrimSpace(req.PushPath) == "" {
		return Result{}, ErrPushPathRequired
	}
	if strings.TrimSpace(req.Pusher) == "" {
		return Result{}, ErrPusherRequired
	}
	if strings.TrimSpace(req.TaskRef) == "" {
		return Result{}, ErrTaskRefRequired
	}

	abs, err := projectfs.SafeJoin(req.LocalBase, rel)
	if err != nil {
		return Result{}, err
	}
	info, err := os.Stat(abs)
	if err != nil {
		if os.IsNotExist(err) {
			return Result{}, ErrNotFound
		}
		return Result{}, err
	}

	pushTime := time.Now()

	conn, err := ftpclient.OpenSession(req.FTPConfig)
	if err != nil {
		return Result{}, err
	}
	defer conn.Quit()

	var uploadLocal string
	var uploadName string
	var cleanup func()
	if info.IsDir() {
		tmp, name, clean, err := zipDirToTemp(abs)
		if err != nil {
			return Result{}, err
		}
		uploadLocal = tmp
		uploadName = name
		cleanup = clean
	} else {
		uploadLocal = abs
		uploadName = filepath.Base(abs)
		cleanup = func() {}
	}
	defer cleanup()

	layout := ResolvePushLayout(req.PushPath, uploadName)
	backupDir := strings.TrimSpace(req.BackupPath)
	if backupDir == "" {
		backupDir = DefaultBackupPath(layout.PushDir, pushTime)
	} else {
		backupDir = normalizeRemoteDir(backupDir)
	}

	updatePath := layout.UpdatePath
	backupFilePath := UpdateRemotePath(backupDir, uploadName)
	infoRemote := UpdateRemotePath(layout.PushDir, InfoFileName)
	backupInfoPath := UpdateRemotePath(backupDir, InfoFileName)
	manifestBackup := ""

	exists, err := ftpclient.RemoteExists(conn, req.FTPConfig, updatePath)
	if err != nil {
		return Result{}, fmt.Errorf("check remote: %w", err)
	}
	if exists {
		if err := ftpclient.BackupRemoteFile(conn, req.FTPConfig, updatePath, backupFilePath); err != nil {
			return Result{}, fmt.Errorf("backup remote file: %w", err)
		}
		manifestBackup = backupFilePath
	}

	noteExists, err := ftpclient.RemoteExists(conn, req.FTPConfig, infoRemote)
	if err != nil {
		return Result{}, fmt.Errorf("check remote note: %w", err)
	}
	if noteExists {
		if err := ftpclient.BackupRemoteFile(conn, req.FTPConfig, infoRemote, backupInfoPath); err != nil {
			return Result{}, fmt.Errorf("backup remote note: %w", err)
		}
	}

	if err := ftpclient.UploadLocalFile(conn, req.FTPConfig, updatePath, uploadLocal); err != nil {
		return Result{}, fmt.Errorf("upload update package: %w", err)
	}

	manifest := BuildManifestContent(ManifestInput{
		Pusher:   req.Pusher,
		TaskRef:  req.TaskRef,
		Detail:   req.Detail,
		PushTime: pushTime,
	}, backupFilePath, updatePath)
	if err := ftpclient.UploadBytes(conn, req.FTPConfig, strings.TrimPrefix(infoRemote, "/"), []byte(manifest)); err != nil {
		return Result{}, fmt.Errorf("upload info file: %w", err)
	}

	return Result{
		BackupPath:   manifestBackup,
		UpdatePath:   updatePath,
		InfoFilePath: infoRemote,
		InfoContent:  manifest,
	}, nil
}

// ParseFTPConfig unmarshals env config_json into FTPConfig.
func ParseFTPConfig(raw string) (envtypes.FTPConfig, error) {
	var cfg envtypes.FTPConfig
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		return cfg, fmt.Errorf("ftp config: %w", err)
	}
	return cfg, nil
}
