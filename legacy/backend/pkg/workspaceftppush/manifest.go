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

// NotePathPair is one package's backup/update paths inside a merged updateNote block.
type NotePathPair struct {
	BackupPath string `json:"backup_path"`
	UpdatePath string `json:"update_path"`
}

// BuildManifestContent returns the auto-update description file (updateNote.txt).
// Format:
//
//	{YYYYMMDD} {pusher} {task description}
//	备份包路径:{backup path}
//	更新包路径:{update path}
//	{detail — module/file list, optional}
func BuildManifestContent(in ManifestInput, backupPath, updatePath string) string {
	return BuildManifestContentPairs(in, []NotePathPair{{
		BackupPath: backupPath,
		UpdatePath: updatePath,
	}})
}

// BuildManifestContentPairs builds one updateNote block with multiple package path pairs.
// Multiple packages share one 备份包路径 / 更新包路径 line, joined by ";".
func BuildManifestContentPairs(in ManifestInput, pairs []NotePathPair) string {
	ts := in.PushTime
	if ts.IsZero() {
		ts = time.Now()
	}
	line1 := fmt.Sprintf("%s %s %s",
		ts.Format("20060102"),
		strings.TrimSpace(in.Pusher),
		strings.TrimSpace(in.TaskRef),
	)
	var backups, updates []string
	for _, p := range pairs {
		backup := strings.TrimSpace(p.BackupPath)
		update := strings.TrimSpace(p.UpdatePath)
		if backup == "" && update == "" {
			continue
		}
		if backup != "" {
			backups = append(backups, backup)
		}
		if update != "" {
			updates = append(updates, update)
		}
	}
	lines := []string{
		line1,
		backupPathLabel + strings.Join(backups, ";"),
		updatePathLabel + strings.Join(updates, ";"),
	}
	if detail := formatManifestDetail(in.Detail); detail != "" {
		lines = append(lines, detail)
	}
	return strings.Join(lines, "\n") + "\n"
}

// formatManifestDetail indents each detail line with 4 spaces (ops updateNote convention).
func formatManifestDetail(raw string) string {
	raw = strings.TrimRight(strings.ReplaceAll(raw, "\r\n", "\n"), "\n")
	if strings.TrimSpace(raw) == "" {
		return ""
	}
	parts := strings.Split(raw, "\n")
	out := make([]string, 0, len(parts))
	for _, line := range parts {
		trimmed := strings.TrimRight(line, " \t")
		if strings.TrimSpace(trimmed) == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "    ") || strings.HasPrefix(trimmed, "\t") {
			out = append(out, trimmed)
			continue
		}
		out = append(out, "    "+strings.TrimLeft(trimmed, " \t"))
	}
	return strings.Join(out, "\n")
}

// MergeUpdateNoteContent appends a new manifest block after existing remote note text.
// When appending to non-empty content, inserts a blank line before the new block.
func MergeUpdateNoteContent(existing, manifest string) string {
	manifest = strings.TrimRight(strings.ReplaceAll(manifest, "\r\n", "\n"), "\n")
	if manifest == "" {
		return strings.TrimRight(strings.ReplaceAll(existing, "\r\n", "\n"), "\n") + "\n"
	}
	existing = strings.TrimRight(strings.ReplaceAll(existing, "\r\n", "\n"), "\n")
	if existing == "" {
		return manifest + "\n"
	}
	return existing + "\n\n" + manifest + "\n"
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
