package workspaceftppush

import (
	"fmt"
	"path"
	"strings"
	"time"

	"auto-pack/backend/pkg/ftpclient"
)

const InfoFileName = "updateNote.txt"

const (
	backupPathLabel = "备份包路径:"
	updatePathLabel = "更新包路径:"
)

// ManifestInput holds user-provided push metadata.
type ManifestInput struct {
	Pusher   string
	TaskRef  string
	Detail   string
	PushTime time.Time
}

// BuildManifestContent returns the auto-update description file (updateNote.txt).
// Format:
//
//	{YYYYMMDD} {pusher} {task description}
//	备份包路径:{backup path}
//	更新包路径:{update path}
//	{detail — module/file list, optional}
func BuildManifestContent(in ManifestInput, backupPath, updatePath string) string {
	ts := in.PushTime
	if ts.IsZero() {
		ts = time.Now()
	}
	line1 := fmt.Sprintf("%s %s %s",
		ts.Format("20060102"),
		strings.TrimSpace(in.Pusher),
		strings.TrimSpace(in.TaskRef),
	)
	lines := []string{
		line1,
		backupPathLabel + strings.TrimSpace(backupPath),
		updatePathLabel + strings.TrimSpace(updatePath),
	}
	detail := strings.TrimRight(strings.ReplaceAll(in.Detail, "\r\n", "\n"), "\n")
	if strings.TrimSpace(detail) != "" {
		lines = append(lines, detail)
	}
	return strings.Join(lines, "\n") + "\n"
}

// MergeUpdateNoteContent appends a new manifest block after existing remote note text.
func MergeUpdateNoteContent(existing, manifest string) string {
	manifest = strings.TrimRight(strings.ReplaceAll(manifest, "\r\n", "\n"), "\n")
	if manifest == "" {
		return strings.TrimRight(strings.ReplaceAll(existing, "\r\n", "\n"), "\n") + "\n"
	}
	existing = strings.TrimRight(strings.ReplaceAll(existing, "\r\n", "\n"), "\n")
	if existing == "" {
		return manifest + "\n"
	}
	return existing + "\n" + manifest + "\n"
}

// PushLayout holds resolved remote paths after interpreting push_path.
type PushLayout struct {
	PushDir    string // parent directory for updateNote.txt and default BAK
	UpdatePath string // full remote path of the uploaded package
}

// ResolvePushLayout treats pushPath as a remote directory, unless its last segment
// matches uploadName—in which case pushPath is the full remote file path.
func ResolvePushLayout(pushPath, uploadName string) PushLayout {
	pushPath = strings.TrimSpace(strings.ReplaceAll(pushPath, "\\", "/"))
	uploadName = strings.TrimSpace(uploadName)
	trimmed := strings.TrimRight(pushPath, "/")
	if trimmed == "" {
		trimmed = "/"
	}
	if !strings.HasPrefix(trimmed, "/") {
		trimmed = "/" + trimmed
	}
	if uploadName != "" && path.Base(trimmed) == uploadName {
		dir := path.Dir(trimmed)
		if dir == "." || dir == "" {
			dir = "/"
		}
		return PushLayout{
			PushDir:    normalizeRemoteDir(dir),
			UpdatePath: trimmed,
		}
	}
	pushDir := normalizeRemoteDir(pushPath)
	return PushLayout{
		PushDir:    pushDir,
		UpdatePath: UpdateRemotePath(pushDir, uploadName),
	}
}

// BackupDirStamp returns YYYYMMDDHHmmssSSS for BAK subdirectory names.
func BackupDirStamp(ts time.Time) string {
	if ts.IsZero() {
		ts = time.Now()
	}
	return ts.Format("20060102150405") + fmt.Sprintf("%03d", ts.Nanosecond()/1e6)
}

// DefaultBackupPath appends BAK/{timestamp} under pushDir.
func DefaultBackupPath(pushDir string, ts time.Time) string {
	pushDir = normalizeRemoteDir(pushDir)
	return pushDir + "/BAK/" + BackupDirStamp(ts)
}

func normalizeRemoteDir(p string) string {
	p = strings.TrimSpace(strings.ReplaceAll(p, "\\", "/"))
	p = strings.TrimRight(p, "/")
	if p == "" {
		return "/"
	}
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	return p
}

// UpdateRemotePath joins push directory with uploaded file name (POSIX, leading slash).
func UpdateRemotePath(pushPath, fileName string) string {
	return ftpclient.LocalRelToRemote(normalizeRemoteDir(pushPath), fileName)
}
