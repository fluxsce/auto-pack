package workspaceftppush

import (
	"fmt"
	"strings"
	"time"

	"auto-pack/backend/pkg/envtypes"
	"auto-pack/backend/pkg/ftpclient"
)

// AppendUpdateNoteRequest writes one merged updateNote.txt block without uploading a package.
type AppendUpdateNoteRequest struct {
	FTPConfig envtypes.FTPConfig
	PushPath  string
	Pusher    string
	TaskRef   string
	Detail    string
	Paths     []NotePathPair
	PushTime  time.Time
	Logf      func(format string, args ...any)
}

func (r AppendUpdateNoteRequest) logf(format string, args ...any) {
	if r.Logf != nil {
		r.Logf(format, args...)
	}
}

// AppendUpdateNote appends (or creates) a single merged updateNote.txt entry.
func AppendUpdateNote(req AppendUpdateNoteRequest) (Result, error) {
	if strings.TrimSpace(req.PushPath) == "" {
		return Result{}, ErrPushPathRequired
	}
	if strings.TrimSpace(req.Pusher) == "" {
		return Result{}, ErrPusherRequired
	}
	if strings.TrimSpace(req.TaskRef) == "" {
		return Result{}, ErrTaskRefRequired
	}
	pairs := make([]NotePathPair, 0, len(req.Paths))
	for _, p := range req.Paths {
		backup := strings.TrimSpace(p.BackupPath)
		update := strings.TrimSpace(p.UpdatePath)
		if backup == "" && update == "" {
			continue
		}
		pairs = append(pairs, NotePathPair{BackupPath: backup, UpdatePath: update})
	}
	if len(pairs) == 0 {
		return Result{}, fmt.Errorf("at least one note path pair is required")
	}

	// Use first update file name only to resolve push directory from push_path.
	sampleName := ""
	if u := pairs[0].UpdatePath; u != "" {
		parts := strings.Split(strings.ReplaceAll(u, "\\", "/"), "/")
		sampleName = parts[len(parts)-1]
	}
	layout := ResolvePushLayout(req.PushPath, sampleName)
	infoRemote := UpdateRemotePath(layout.PushDir, InfoFileName)

	pushTime := req.PushTime
	if pushTime.IsZero() {
		pushTime = time.Now()
	}
	manifest := BuildManifestContentPairs(ManifestInput{
		Pusher:   req.Pusher,
		TaskRef:  req.TaskRef,
		Detail:   req.Detail,
		PushTime: pushTime,
	}, pairs)

	conn, err := ftpclient.OpenSession(req.FTPConfig)
	if err != nil {
		return Result{}, err
	}
	defer conn.Quit()

	noteExists, err := ftpclient.RemoteExists(conn, req.FTPConfig, infoRemote)
	if err != nil {
		return Result{}, fmt.Errorf("check remote note: %w", err)
	}
	noteUpload := manifest
	if noteExists {
		req.logf("Remote updateNote.txt exists; appending merged record")
		existing, err := ftpclient.DownloadRemote(conn, req.FTPConfig, infoRemote)
		if err != nil {
			return Result{}, fmt.Errorf("read remote note: %w", err)
		}
		noteUpload = MergeUpdateNoteContent(string(existing), manifest)
	} else {
		req.logf("Remote updateNote.txt not found; creating merged note")
	}
	req.logf("Writing merged updateNote.txt for %d packages", len(pairs))
	if err := ftpclient.UploadBytes(conn, req.FTPConfig, infoRemote, []byte(noteUpload)); err != nil {
		return Result{}, fmt.Errorf("upload info file: %w", err)
	}

	return Result{
		BackupPath:   pairs[0].BackupPath,
		UpdatePath:   pairs[len(pairs)-1].UpdatePath,
		InfoFilePath: infoRemote,
		InfoContent:  noteUpload,
	}, nil
}
