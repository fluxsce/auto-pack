package projectrelease

import (
	"encoding/json"
	"strings"
)

// PostOptions is stored on AP_PACK_WORKSPACE_RELEASE for multi-user auto-release UI restore.
type PostOptions struct {
	AutoBuildAfterRelease bool     `json:"auto_build_after_release"`
	AutoFtpPushAfterBuild bool     `json:"auto_ftp_push_after_build"`
	FtpPushProjectIDs     []string `json:"ftp_push_project_ids"`
	FtpEnvRecID           string   `json:"ftp_env_rec_id"`
	PushPath              string   `json:"push_path"`
	BackupPath            string   `json:"backup_path"`
	BackupManual          bool     `json:"backup_manual"`
	Pusher                string   `json:"pusher"`
	TaskRef               string   `json:"task_ref"`
	Detail                string   `json:"detail"`
	ReleaseDate           string   `json:"release_date,omitempty"`
	UpdatePomVersion      *bool    `json:"update_pom_version,omitempty"`
	CreateTargetBranch    *bool    `json:"create_target_branch,omitempty"`
	TargetBranch          string   `json:"target_branch,omitempty"`
}

func ParsePostOptionsJSON(raw string) (*PostOptions, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	var opts PostOptions
	if err := json.Unmarshal([]byte(raw), &opts); err != nil {
		return nil, err
	}
	return &opts, nil
}

func MarshalPostOptions(opts *PostOptions) (string, error) {
	if opts == nil {
		return "", nil
	}
	b, err := json.Marshal(opts)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
