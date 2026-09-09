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

// SourcePath is one local file or directory to include in a merged zip.
type SourcePath struct {
	LocalBase    string
	RelativePath string
}

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
	// RelativePaths are extra paths under LocalBase; used with MergeZipRoot.
	RelativePaths []string
	// MergeSources are extra paths that may live in other project work dirs.
	MergeSources []SourcePath
	// MergeZipRoot, when set and there are 2+ sources, packs them into one zip
	// named {MergeZipRoot}.zip. Each source keeps its original directory name.
	MergeZipRoot string
	Logf         func(format string, args ...any)
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
	if (rel == "" || rel == ".") && strings.TrimSpace(req.MergeZipRoot) == "" {
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

	pushTime := time.Now()

	conn, err := ftpclient.OpenSession(req.FTPConfig)
	if err != nil {
		return Result{}, err
	}
	defer conn.Quit()

	uploadLocal, uploadName, cleanup, err := prepareUpload(req, rel)
	if err != nil {
		return Result{}, err
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

	req.logf("Ensuring remote directory: %s", layout.PushDir)
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
	if err := ftpclient.UploadBytes(conn, req.FTPConfig, infoRemote, []byte(noteUpload)); err != nil {
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

func prepareUpload(req Request, rel string) (uploadLocal, uploadName string, cleanup func(), err error) {
	root := strings.TrimSpace(req.MergeZipRoot)
	if root != "" {
		absPaths, err := resolveMergeAbsPaths(req, rel)
		if err != nil {
			return "", "", nil, err
		}
		if len(absPaths) >= 2 {
			req.logf("Merging %d path(s) into %s.zip (original directory names kept)", len(absPaths), sanitizeZipRoot(root))
			for _, p := range absPaths {
				req.logf("  source: %s", p)
			}
			tmp, name, clean, err := zipSourcesToTemp(absPaths, root, req.logf)
			if err != nil {
				return "", "", nil, err
			}
			req.logf("Created merged temp package: %s", name)
			return tmp, name, clean, nil
		}
		if len(absPaths) == 1 {
			req.logf("Only one source; skip merge zip and push as a single item")
			return uploadSingleLocal(req, absPaths[0])
		}
	}

	abs, err := projectfs.SafeJoin(req.LocalBase, rel)
	if err != nil {
		return "", "", nil, err
	}
	return uploadSingleLocal(req, abs)
}

func uploadSingleLocal(req Request, abs string) (uploadLocal, uploadName string, cleanup func(), err error) {
	req.logf("Resolved local path: %s", abs)
	info, err := os.Stat(abs)
	if err != nil {
		if os.IsNotExist(err) {
			return "", "", nil, ErrNotFound
		}
		return "", "", nil, err
	}
	if info.IsDir() {
		req.logf("Directory detected; packaging as zip before upload")
		tmp, name, clean, err := zipDirToTemp(abs)
		if err != nil {
			return "", "", nil, err
		}
		req.logf("Created temp package: %s", name)
		return tmp, name, clean, nil
	}
	req.logf("File detected; uploading directly: %s", filepath.Base(abs))
	return abs, filepath.Base(abs), func() {}, nil
}

func resolveMergeAbsPaths(req Request, rel string) ([]string, error) {
	var absPaths []string
	rels := make([]string, 0, 1+len(req.RelativePaths))
	if rel != "" && rel != "." {
		rels = append(rels, rel)
	}
	for _, extra := range req.RelativePaths {
		extra = strings.TrimSpace(strings.ReplaceAll(extra, "\\", "/"))
		if extra == "" || extra == "." || extra == rel {
			continue
		}
		rels = append(rels, extra)
	}
	for _, item := range rels {
		abs, err := projectfs.SafeJoin(req.LocalBase, item)
		if err != nil {
			return nil, err
		}
		if _, err := os.Stat(abs); err != nil {
			if os.IsNotExist(err) {
				return nil, ErrNotFound
			}
			return nil, err
		}
		absPaths = append(absPaths, abs)
	}
	for _, src := range req.MergeSources {
		srcRel := strings.TrimSpace(strings.ReplaceAll(src.RelativePath, "\\", "/"))
		if srcRel == "" || srcRel == "." {
			return nil, ErrPathRequired
		}
		abs, err := projectfs.SafeJoin(src.LocalBase, srcRel)
		if err != nil {
			return nil, err
		}
		if _, err := os.Stat(abs); err != nil {
			if os.IsNotExist(err) {
				return nil, ErrNotFound
			}
			return nil, err
		}
		absPaths = append(absPaths, abs)
	}
	if len(absPaths) == 0 {
		return nil, ErrPathRequired
	}
	return uniqueStrings(absPaths), nil
}

func uniqueStrings(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}
