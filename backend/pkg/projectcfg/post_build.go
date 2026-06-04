package projectcfg

import "strings"

// Post-build action types (stored in config_json.post_build_actions).
const (
	PostBuildShellExec   = "shell_exec"
	PostBuildSSHExec     = "ssh_exec"
	PostBuildSSHTransfer = "ssh_transfer"
	PostBuildEmailNotify = "email_notify"
)

// PostBuildWhen controls when an action runs relative to build result.
type PostBuildWhen string

const (
	PostBuildWhenSuccess = "success"
	PostBuildWhenFailure = "failure"
	PostBuildWhenAlways  = "always"
)

// PostBuildTransferSet mirrors Jenkins transfer set fields.
type PostBuildTransferSet struct {
	SourceFiles  string `json:"source_files"`
	RemovePrefix string `json:"remove_prefix"`
	RemoteDir    string `json:"remote_dir"`
	ExecCommand  string `json:"exec_command"`
}

// PostBuildAction is one post-build step configured on a project.
type PostBuildAction struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Type     string `json:"type"`
	Enabled  bool   `json:"enabled"`
	When     string `json:"when"`
	EnvRecID string `json:"env_rec_id"`

	Command string `json:"command,omitempty"`

	TransferSets []PostBuildTransferSet `json:"transfer_sets,omitempty"`

	Recipients string `json:"recipients,omitempty"`
	Subject    string `json:"subject,omitempty"`
	Body       string `json:"body,omitempty"`
}

// ShouldRun reports whether the action applies to the given build outcome.
func (a PostBuildAction) ShouldRun(buildSucceeded bool) bool {
	if !a.Enabled {
		return false
	}
	switch PostBuildWhen(strings.TrimSpace(a.When)) {
	case PostBuildWhenAlways:
		return true
	case PostBuildWhenFailure:
		return !buildSucceeded
	case PostBuildWhenSuccess, "":
		return buildSucceeded
	default:
		return buildSucceeded
	}
}

// EffectivePostBuildActions returns configured actions or a legacy deploy env transfer step.
func EffectivePostBuildActions(actions []PostBuildAction, deployEnvRecID string) []PostBuildAction {
	if len(actions) > 0 {
		out := make([]PostBuildAction, 0, len(actions))
		for _, a := range actions {
			if strings.TrimSpace(a.Type) == "" {
				continue
			}
			out = append(out, a)
		}
		return out
	}
	id := strings.TrimSpace(deployEnvRecID)
	if id == "" {
		return nil
	}
	return []PostBuildAction{{
		Type:     PostBuildSSHTransfer,
		Enabled:  true,
		When:     string(PostBuildWhenSuccess),
		EnvRecID: id,
		TransferSets: []PostBuildTransferSet{{
			SourceFiles:  "**/*",
			RemovePrefix: "target",
			RemoteDir:    "/",
		}},
	}}
}
