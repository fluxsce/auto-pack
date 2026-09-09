package email

import (
	"errors"
	"fmt"
	"strings"
)

// Message is an outbound email.
type Message struct {
	To          []string
	Cc          []string
	Bcc         []string
	Subject     string
	Text        string
	HTML        string
	Attachments []Attachment
}

// Attachment is a file attached to the message.
type Attachment struct {
	Filename    string
	ContentType string
	Data        []byte
}

func (m *Message) normalize() {
	trimSlice := func(ss []string) []string {
		out := make([]string, 0, len(ss))
		for _, s := range ss {
			s = strings.TrimSpace(s)
			if s != "" {
				out = append(out, s)
			}
		}
		return out
	}
	m.To = trimSlice(m.To)
	m.Cc = trimSlice(m.Cc)
	m.Bcc = trimSlice(m.Bcc)
	m.Subject = strings.TrimSpace(m.Subject)
	m.Text = strings.TrimSpace(m.Text)
	m.HTML = strings.TrimSpace(m.HTML)
}

// Validate checks recipients and body content.
func (m Message) Validate() error {
	msg := m
	msg.normalize()
	if len(msg.To) == 0 {
		return errors.New("at least one To recipient is required")
	}
	if err := ValidateAddresses(msg.To); err != nil {
		return fmt.Errorf("to: %w", err)
	}
	if err := ValidateAddresses(msg.Cc); err != nil {
		return fmt.Errorf("cc: %w", err)
	}
	if err := ValidateAddresses(msg.Bcc); err != nil {
		return fmt.Errorf("bcc: %w", err)
	}
	if msg.Subject == "" {
		return errors.New("subject is required")
	}
	if msg.Text == "" && msg.HTML == "" && len(msg.Attachments) == 0 {
		return errors.New("message body or attachment is required")
	}
	for i, a := range msg.Attachments {
		if strings.TrimSpace(a.Filename) == "" {
			return fmt.Errorf("attachment[%d]: filename is required", i)
		}
		if len(a.Data) == 0 {
			return fmt.Errorf("attachment[%d]: data is empty", i)
		}
	}
	return nil
}

// AllRecipients returns To, Cc, and Bcc combined for SMTP RCPT.
func (m Message) AllRecipients() []string {
	msg := m
	msg.normalize()
	n := len(msg.To) + len(msg.Cc) + len(msg.Bcc)
	out := make([]string, 0, n)
	out = append(out, msg.To...)
	out = append(out, msg.Cc...)
	out = append(out, msg.Bcc...)
	return out
}
