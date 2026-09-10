package env

import (
	"context"
	"errors"
	"strings"

	"auto-pack/server/internal/host/smtp"
	"auto-pack/server/internal/iam"
)

var (
	// ErrMailSMTP is returned when the env is not smtp.
	ErrMailSMTP = errors.New("environment must be smtp")
	// ErrMailDisabled is returned when the env is disabled.
	ErrMailDisabled = errors.New("env setting is disabled")
	// ErrMailRecipients is returned when no recipients are given.
	ErrMailRecipients = errors.New("recipients is required")
	// ErrMailSubject is returned when the subject is empty.
	ErrMailSubject = errors.New("subject is required")
	// ErrMailBody is returned when both text and html are empty.
	ErrMailBody = errors.New("body or html is required")
)

// MailIn is a one-off notify sent through a saved SMTP env (env.write).
type MailIn struct {
	Recipients string `json:"recipients"`
	Subject    string `json:"subject"`
	Body       string `json:"body"`
	HTML       string `json:"html,omitempty"`
	FromName   string `json:"from_name,omitempty"`
}

// SendMail delivers one message via a saved SMTP env (env.write).
func (s *Service) SendMail(ctx context.Context, actor iam.Actor, recID string, in MailIn) error {
	e, cfg, err := s.DecryptRow(ctx, recID)
	if err != nil {
		return err
	}
	if err := s.authorizeWrite(ctx, actor, e.WorkspaceRecID); err != nil {
		return err
	}
	kind, err := NormalizeKind(e.EnvKind)
	if err != nil || kind != "smtp" {
		return ErrMailSMTP
	}
	if e.IsEnabled == 0 {
		return ErrMailDisabled
	}
	recipients := strings.TrimSpace(in.Recipients)
	if recipients == "" {
		return ErrMailRecipients
	}
	subject := strings.TrimSpace(in.Subject)
	text := strings.TrimSpace(in.Body)
	html := strings.TrimSpace(in.HTML)
	if subject == "" {
		return ErrMailSubject
	}
	if text == "" && html == "" {
		return ErrMailBody
	}
	sc := smtp.MapConfig(cfg)
	sc.To = recipients
	if name := strings.TrimSpace(in.FromName); name != "" {
		sc.FromName = name
	}
	return smtp.SendMIME(sc, subject, text, html)
}
