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
	// SkipUpdateNote uploads the package but does not write updateNote.txt.
	SkipUpdateNote bool
	// PriorNotePaths are earlier packages in the same batch; merged into one note on write.
	PriorNotePaths []NotePathPair
	Logf           func(format string, args ...any)
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

func (r Request) logf(format string, args ...any) {
	if r.Logf != nil {
		r.Logf(format, args...)
	}
}

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
	req.logf("Resolved local path: %s", abs)
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
		req.logf("Directory detected; packaging as zip before upload")
		tmp, name, clean, err := zipDirToTemp(abs)
		if err != nil {
			return Result{}, err
		}
		uploadLocal = tmp
		uploadName = name
		cleanup = clean
		req.logf("Created temp package: %s", uploadName)
	} else {
		uploadLocal = abs
		uploadName = filepath.Base(abs)
		cleanup = func() {}
		req.logf("File detected; uploading directly: %s", uploadName)
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
	manifestBackup := ""
	req.logf("Resolved remote update path: %s", updatePath)
	req.logf("Resolved remote backup path: %s", backupFilePath)

	exists, err := ftpclient.RemoteExists(conn, req.FTPConfig, updatePath)
	if err != nil {
		return Result{}, fmt.Errorf("check remote: %w", err)
	}
	if exists {
		req.logf("Remote target exists; backing up previous file")
		if err := ftpclient.BackupRemoteFile(conn, req.FTPConfig, updatePath, backupFilePath); err != nil {
			return Result{}, fmt.Errorf("backup remote file: %w", err)
		}
		manifestBackup = backupFilePath
		req.logf("Remote backup completed")
	} else {
		req.logf("Remote target does not exist; backup skipped")
	}

	req.logf("Uploading package to remote")
	if err := ftpclient.UploadLocalFile(conn, req.FTPConfig, updatePath, uploadLocal); err != nil {
		return Result{}, fmt.Errorf("upload update package: %w", err)
	}
	req.logf("Package upload completed")

	if req.SkipUpdateNote {
		req.logf("Skip writing updateNote.txt (batch merge later)")
		return Result{
			BackupPath:   manifestBackup,
			UpdatePath:   updatePath,
			InfoFilePath: infoRemote,
			InfoContent:  "",
		}, nil
	}

	pairs := make([]NotePathPair, 0, len(req.PriorNotePaths)+1)
	for _, p := range req.PriorNotePaths {
		pairs = append(pairs, NotePathPair{
			BackupPath: strings.TrimSpace(p.BackupPath),
			UpdatePath: strings.TrimSpace(p.UpdatePath),
		})
	}
	pairs = append(pairs, NotePathPair{
		BackupPath: backupFilePath,
		UpdatePath: updatePath,
	})
	manifest := BuildManifestContentPairs(ManifestInput{
		Pusher:   req.Pusher,
		TaskRef:  req.TaskRef,
		Detail:   req.Detail,
		PushTime: pushTime,
	}, pairs)

	noteExists, err := ftpclient.RemoteExists(conn, req.FTPConfig, infoRemote)
	if err != nil {
		return Result{}, fmt.Errorf("check remote note: %w", err)
	}
	noteUpload := manifest
	if noteExists {
		req.logf("Remote updateNote.txt exists; appending current record")
		existing, err := ftpclient.DownloadRemote(conn, req.FTPConfig, infoRemote)
		if err != nil {
			return Result{}, fmt.Errorf("read remote note: %w", err)
		}
		noteUpload = MergeUpdateNoteContent(string(existing), manifest)
	} else {
		req.logf("Remote updateNote.txt not found; creating a new one")
	}
	if len(pairs) > 1 {
		req.logf("Writing merged updateNote.txt for %d packages", len(pairs))
	}
	if err := ftpclient.UploadBytes(conn, req.FTPConfig, strings.TrimPrefix(infoRemote, "/"), []byte(noteUpload)); err != nil {
		return Result{}, fmt.Errorf("upload info file: %w", err)
	}
	req.logf("updateNote.txt upload completed")

	return Result{
		BackupPath:   manifestBackup,
		UpdatePath:   updatePath,
		InfoFilePath: infoRemote,
		InfoContent:  noteUpload,
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
