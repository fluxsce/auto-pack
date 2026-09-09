package projectcfg

import (
	"errors"
	"strings"

	"github.com/google/uuid"
)

// EmailNotifyBulkOpts is the template applied to each project's post_build_actions.
type EmailNotifyBulkOpts struct {
	Enabled    bool
	When       string
	EnvRecID   string
	Recipients string
	Subject    string
	Body       string
	Name       string
}

// ValidateEmailNotifyBulk checks bulk email notify options.
func ValidateEmailNotifyBulk(opts EmailNotifyBulkOpts) error {
	if !opts.Enabled {
		return nil
	}
	if strings.TrimSpace(opts.Recipients) == "" {
		return errors.New("recipients is required when email notify is enabled")
	}
	when := strings.TrimSpace(opts.When)
	if when != "" &&
		when != string(PostBuildWhenSuccess) &&
		when != string(PostBuildWhenFailure) &&
		when != string(PostBuildWhenAlways) {
		return errors.New("invalid when for email notify")
	}
	return nil
}

// ApplyEmailNotifyToConfig upserts one email_notify action in cfg.PostBuildActions.
func ApplyEmailNotifyToConfig(cfg *Config, opts EmailNotifyBulkOpts) {
	when := strings.TrimSpace(opts.When)
	if when == "" {
		when = string(PostBuildWhenFailure)
	}
	name := strings.TrimSpace(opts.Name)
	if name == "" {
		name = "Build email"
	}
	action := PostBuildAction{
		Type:       PostBuildEmailNotify,
		Enabled:    opts.Enabled,
		When:       when,
		EnvRecID:   strings.TrimSpace(opts.EnvRecID),
		Recipients: strings.TrimSpace(opts.Recipients),
		Subject:    strings.TrimSpace(opts.Subject),
		Body:       strings.TrimSpace(opts.Body),
		Name:       name,
	}
	for i := range cfg.PostBuildActions {
		if strings.TrimSpace(cfg.PostBuildActions[i].Type) == PostBuildEmailNotify {
			action.ID = cfg.PostBuildActions[i].ID
			if strings.TrimSpace(cfg.PostBuildActions[i].Name) != "" {
				action.Name = cfg.PostBuildActions[i].Name
			}
			cfg.PostBuildActions[i] = action
			return
		}
	}
	action.ID = uuid.New().String()
	cfg.PostBuildActions = append(cfg.PostBuildActions, action)
}
