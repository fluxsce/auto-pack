package env

import (
	"context"
	"errors"
	"fmt"
	"html"
	"strings"
	"time"

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
	// ErrMailTemplate is returned when template is unknown.
	ErrMailTemplate = errors.New("unknown mail template")
)

// Built-in notify templates for env.SendMail.
const (
	// MailTemplateFTPUpdate is the DATAHUB FTP push notify (existing layout).
	MailTemplateFTPUpdate = "ftp_update"
	// MailTemplateProjectUpdate is the manual project-update notify.
	MailTemplateProjectUpdate = "project_update"
)

// UpdateMailFields fills ftp_update / project_update templates.
type UpdateMailFields struct {
	ProjectLabel        string `json:"project_label"`
	UpdateContent       string `json:"update_content"`
	TaskNo              string `json:"task_no"`
	FTPVersionSource    string `json:"ftp_version_source"`
	UpdateMethod        string `json:"update_method"`
	NeedRestart         string `json:"need_restart"`
	BrowserCacheRefresh string `json:"browser_cache_refresh"`
	ImpactScope         string `json:"impact_scope"`
	UpdateScope         string `json:"update_scope"`
	AffectsAPI          string `json:"affects_api"`
	WarPackage          string `json:"war_package"`
	Other               string `json:"other"`
	Backup              string `json:"backup"`
}

// MailIn is a one-off notify sent through a saved SMTP env (env.write).
// When Template is set, subject/body/html are composed from Fields
// (Subject may still override the generated subject).
type MailIn struct {
	Recipients string           `json:"recipients"`
	Subject    string           `json:"subject"`
	Body       string           `json:"body"`
	HTML       string           `json:"html,omitempty"`
	FromName   string           `json:"from_name,omitempty"`
	Template   string           `json:"template,omitempty"`
	Fields     UpdateMailFields `json:"fields,omitempty"`
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
	htmlBody := strings.TrimSpace(in.HTML)
	fromName := strings.TrimSpace(in.FromName)

	if tpl := strings.TrimSpace(in.Template); tpl != "" {
		composed, err := ComposeUpdateMail(tpl, in.Fields, time.Now())
		if err != nil {
			return err
		}
		if subject == "" {
			subject = composed.Subject
		}
		text = composed.Text
		htmlBody = composed.HTML
		if fromName == "" {
			fromName = composed.FromName
		}
	}

	if subject == "" {
		return ErrMailSubject
	}
	if text == "" && htmlBody == "" {
		return ErrMailBody
	}
	sc := smtp.MapConfig(cfg)
	sc.To = recipients
	if fromName != "" {
		sc.FromName = fromName
	}
	return smtp.SendMIME(sc, subject, text, htmlBody)
}

// ComposedMail is the rendered subject / bodies for a built-in template.
type ComposedMail struct {
	Subject  string
	Text     string
	HTML     string
	FromName string
}

type updateMailKind struct {
	statusPhrase string
	brand        string
	fromName     string
}

func updateMailKindOf(template string) (updateMailKind, error) {
	switch strings.TrimSpace(template) {
	case MailTemplateFTPUpdate:
		return updateMailKind{
			statusPhrase: "FTP已更新",
			brand:        "DATAHUB 更新通知",
			fromName:     "DATAHUB 更新通知",
		}, nil
	case MailTemplateProjectUpdate:
		return updateMailKind{
			statusPhrase: "项目已更新",
			brand:        "项目更新通知",
			fromName:     "项目更新通知",
		}, nil
	default:
		return updateMailKind{}, ErrMailTemplate
	}
}

// ComposeUpdateMail renders ftp_update / project_update subject and bodies.
func ComposeUpdateMail(template string, fields UpdateMailFields, at time.Time) (ComposedMail, error) {
	kind, err := updateMailKindOf(template)
	if err != nil {
		return ComposedMail{}, err
	}
	if at.IsZero() {
		at = time.Now()
	}
	summary := buildUpdateSummary(fields)
	greeting := dayGreeting(at) + "。" + summary
	if summary != "" {
		greeting += ", "
	}
	greeting += kind.statusPhrase

	subject := kind.statusPhrase
	if summary != "" {
		subject = summary + ", " + kind.statusPhrase
	}

	rows := updateMailRows(fields, summary)
	var text strings.Builder
	text.WriteString(greeting)
	text.WriteByte('\n')
	for _, row := range rows {
		fmt.Fprintf(&text, "%s：%s\n", row[0], row[1])
	}

	return ComposedMail{
		Subject:  subject,
		Text:     strings.TrimRight(text.String(), "\n"),
		HTML:     renderUpdateMailHTML(kind.brand, greeting, rows),
		FromName: kind.fromName,
	}, nil
}

func buildUpdateSummary(fields UpdateMailFields) string {
	label := strings.TrimSpace(fields.ProjectLabel)
	content := strings.TrimSpace(fields.UpdateContent)
	switch {
	case label != "" && content != "":
		return "【" + label + "】" + content
	case content != "":
		return content
	case label != "":
		return "【" + label + "】"
	default:
		return ""
	}
}

func dayGreeting(at time.Time) string {
	h := at.Hour()
	switch {
	case h < 12:
		return "上午好"
	case h < 18:
		return "下午好"
	default:
		return "晚上好"
	}
}

func updateMailRows(fields UpdateMailFields, summary string) [][2]string {
	method := strings.TrimSpace(fields.UpdateMethod)
	if method == "" {
		method = "整包更新"
	}
	needRestart := strings.TrimSpace(fields.NeedRestart)
	if needRestart == "" {
		needRestart = "是"
	}
	browserCache := strings.TrimSpace(fields.BrowserCacheRefresh)
	if browserCache == "" {
		browserCache = "是"
	}
	impact := strings.TrimSpace(fields.ImpactScope)
	if impact == "" {
		impact = "全局"
	}
	affects := strings.TrimSpace(fields.AffectsAPI)
	if affects == "" {
		affects = "是"
	}
	rows := [][2]string{
		{"更新内容", summary},
		{"任务编号", strings.TrimSpace(fields.TaskNo)},
		{"FTP版本来源", strings.TrimSpace(fields.FTPVersionSource)},
		{"更新方式", method},
		{"是否重启", needRestart},
		{"浏览器缓存刷新", browserCache},
		{"影响范围", impact},
		{"更新范围", strings.TrimSpace(fields.UpdateScope)},
		{"是否影响接口运行", affects},
		{"更新war包", strings.TrimSpace(fields.WarPackage)},
	}
	if other := strings.TrimSpace(fields.Other); other != "" {
		rows = append(rows, [2]string{"其他", other})
	}
	rows = append(rows, [2]string{"备份", strings.TrimSpace(fields.Backup)})
	return rows
}

var updateHighlightLabels = map[string]struct{}{
	"更新内容":    {},
	"FTP版本来源": {},
	"更新war包":  {},
}

func renderUpdateMailHTML(brand, greeting string, rows [][2]string) string {
	var b strings.Builder
	b.WriteString(`<!DOCTYPE html>
<html lang="zh-CN">
<head><meta charset="utf-8" /><meta name="viewport" content="width=device-width,initial-scale=1" /></head>
<body style="margin:0;padding:0;background:#eef2f7;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI','PingFang SC','Microsoft YaHei',sans-serif;">
  <table role="presentation" cellpadding="0" cellspacing="0" width="100%" style="background:#eef2f7;padding:24px 12px;">
    <tr>
      <td align="center">
        <table role="presentation" cellpadding="0" cellspacing="0" width="640" style="max-width:640px;width:100%;background:#ffffff;border:1px solid #d9e2ec;border-radius:10px;overflow:hidden;">
          <tr>
            <td style="padding:18px 22px;background:#0f4c81;color:#ffffff;">
              <div style="font-size:12px;letter-spacing:0.04em;opacity:0.85;margin-bottom:6px;">`)
	b.WriteString(html.EscapeString(brand))
	b.WriteString(`</div>
              <div style="font-size:18px;font-weight:600;line-height:1.4;">`)
	b.WriteString(html.EscapeString(greeting))
	b.WriteString(`</div>
            </td>
          </tr>
          <tr>
            <td style="padding:0;">
              <table role="presentation" cellpadding="0" cellspacing="0" width="100%" style="border-collapse:collapse;">
`)
	for i, row := range rows {
		label, value := row[0], row[1]
		_, highlight := updateHighlightLabels[label]
		bg := "#ffffff"
		if highlight {
			bg = "#e8f1fa"
		} else if i%2 == 0 {
			bg = "#f7f9fc"
		}
		labelColor := "#5b677a"
		labelWeight := "400"
		border := "1px solid #e6ebf2"
		if highlight {
			labelColor = "#0f4c81"
			labelWeight = "700"
			border = "1px solid #c5d8eb"
		}
		fmt.Fprintf(&b, `  <tr>
  <td style="padding:10px 14px;border-bottom:%s;color:%s;font-size:13px;font-weight:%s;width:128px;vertical-align:top;background:%s;">%s</td>
  <td style="padding:10px 14px;border-bottom:%s;color:#1f2937;font-size:13px;line-height:1.55;word-break:break-all;background:%s;">%s</td>
</tr>
`, border, labelColor, labelWeight, bg, html.EscapeString(label), border, bg, renderUpdateRowValue(label, value, highlight))
	}
	b.WriteString(`              </table>
            </td>
          </tr>
          <tr>
            <td style="padding:14px 22px;background:#f8fafc;color:#7b8794;font-size:12px;border-top:1px solid #e6ebf2;">
              本邮件由自动打包系统发送，请相关责任人尽快处理。
            </td>
          </tr>
        </table>
      </td>
    </tr>
  </table>
</body>
</html>`)
	return b.String()
}

func renderUpdateRowValue(label, value string, highlight bool) string {
	text := strings.TrimSpace(value)
	if text == "" {
		text = "—"
	}
	if label == "更新war包" {
		pkgs := splitPackageNames(text)
		if len(pkgs) > 0 {
			parts := make([]string, 0, len(pkgs))
			for _, pkg := range pkgs {
				parts = append(parts, `<span style="display:inline-block;margin:0 6px 2px 0;padding:2px 8px;border-radius:999px;background:#dbeafe;color:#1e3a8a;font-size:12px;font-weight:700;line-height:1.4;">`+html.EscapeString(pkg)+`</span>`)
			}
			return strings.Join(parts, "")
		}
	}
	escaped := html.EscapeString(text)
	if highlight {
		return `<span style="font-weight:700;color:#0f4c81;">` + escaped + `</span>`
	}
	return escaped
}

func splitPackageNames(raw string) []string {
	replacer := strings.NewReplacer("、", ",", "，", ",", "；", ",", ";", ",")
	parts := strings.Split(replacer.Replace(raw), ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if s := strings.TrimSpace(p); s != "" {
			out = append(out, s)
		}
	}
	return out
}
