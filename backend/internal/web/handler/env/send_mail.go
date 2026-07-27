package env

import (
	"encoding/json"
	"net/http"
	"strings"

	"auto-pack/backend/internal/httpx"
	"auto-pack/backend/pkg/audit"
	"auto-pack/backend/pkg/email"
	"auto-pack/backend/pkg/envtypes"
)

type sendMailPayload struct {
	Recipients string `json:"recipients"`
	Subject    string `json:"subject"`
	Body       string `json:"body"`
	HTML       string `json:"html,omitempty"`
	FromName   string `json:"from_name,omitempty"`
}

func (h *Handler) sendMail(w http.ResponseWriter, r *http.Request) {
	recID := r.PathValue("recID")
	item, err := h.repo.FindByRecID(recID)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	if item.IsEnabled != 1 {
		httpx.WriteError(w, http.StatusBadRequest, "env setting is disabled")
		return
	}
	if item.EnvType != envtypes.TypeSMTP {
		httpx.WriteError(w, http.StatusBadRequest, "environment must be smtp")
		return
	}

	var body sendMailPayload
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	recipients := splitMailRecipients(body.Recipients)
	if len(recipients) == 0 {
		httpx.WriteError(w, http.StatusBadRequest, "recipients is required")
		return
	}
	subject := strings.TrimSpace(body.Subject)
	text := strings.TrimSpace(body.Body)
	htmlBody := strings.TrimSpace(body.HTML)
	if subject == "" {
		httpx.WriteError(w, http.StatusBadRequest, "subject is required")
		return
	}
	if text == "" && htmlBody == "" {
		httpx.WriteError(w, http.StatusBadRequest, "body or html is required")
		return
	}

	var raw envtypes.SMTPConfig
	if err := json.Unmarshal([]byte(item.ConfigJSON), &raw); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid smtp config")
		return
	}
	fromName := strings.TrimSpace(body.FromName)
	if fromName == "" {
		fromName = strings.TrimSpace(raw.FromName)
	}
	smtpCfg := email.Config{
		Host: raw.Host, Port: raw.Port, Username: raw.Username, Password: raw.Password,
		From: raw.From, FromName: fromName, TLS: raw.TLS, StartTLS: raw.StartTLS,
		InsecureSkipVerify: raw.InsecureSkipVerify, TimeoutSec: raw.TimeoutSec,
	}
	client, err := email.New(smtpCfg)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	msg := email.Message{To: recipients, Subject: subject, Text: text, HTML: htmlBody}
	if err := client.Send(r.Context(), msg); err != nil {
		h.audit.Record(r.Context(), r, audit.Entry{
			Action: audit.ActionUpdate, Module: audit.ModuleEnv, TargetType: "env",
			TargetID: recID, TargetName: item.EnvName,
			Summary: "发送告知邮件失败",
			Detail:  err.Error(),
			Success: false,
		})
		httpx.WriteError(w, http.StatusBadGateway, err.Error())
		return
	}
	h.audit.Record(r.Context(), r, audit.Entry{
		Action: audit.ActionUpdate, Module: audit.ModuleEnv, TargetType: "env",
		TargetID: recID, TargetName: item.EnvName,
		Summary: "发送告知邮件 → " + strings.Join(recipients, ", "),
		Detail:  subject,
		Success: true,
	})
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func splitMailRecipients(raw string) []string {
	raw = strings.ReplaceAll(raw, ";", ",")
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		key := strings.ToLower(p)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, p)
	}
	return out
}
