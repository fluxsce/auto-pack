package workspaceftppush

import (
	"fmt"
	"strings"

	"auto-pack/backend/pkg/envtypes"
	"auto-pack/backend/pkg/ftpclient"
)

// RemoteUpdateNote describes the remote updateNote.txt before push.
type RemoteUpdateNote struct {
	Exists   bool   `json:"exists"`
	Content  string `json:"content"`
	InfoPath string `json:"info_path"`
}

// FetchRemoteUpdateNote reads updateNote.txt at the push directory if present.
func FetchRemoteUpdateNote(cfg envtypes.FTPConfig, pushPath, uploadName string) (RemoteUpdateNote, error) {
	pushPath = strings.TrimSpace(pushPath)
	if pushPath == "" {
		return RemoteUpdateNote{}, ErrPushPathRequired
	}
	uploadName = strings.TrimSpace(uploadName)

	conn, err := ftpclient.OpenSession(cfg)
	if err != nil {
		return RemoteUpdateNote{}, err
	}
	defer conn.Quit()

	layout := ResolvePushLayout(pushPath, uploadName)
	infoRemote := UpdateRemotePath(layout.PushDir, InfoFileName)

	exists, err := ftpclient.RemoteExists(conn, cfg, infoRemote)
	if err != nil {
		return RemoteUpdateNote{}, fmt.Errorf("check remote note: %w", err)
	}
	out := RemoteUpdateNote{Exists: exists, InfoPath: infoRemote}
	if !exists {
		return out, nil
	}
	data, err := ftpclient.DownloadRemote(conn, cfg, infoRemote)
	if err != nil {
		return RemoteUpdateNote{}, fmt.Errorf("read remote note: %w", err)
	}
	out.Content = string(data)
	return out, nil
}
