package intake

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"auto-pack/server/internal/env"
	hostoss "auto-pack/server/internal/host/oss"
	hostsmtp "auto-pack/server/internal/host/smtp"
	"auto-pack/server/internal/project"
)

func (s *Service) mqConfig(ctx context.Context) map[string]any {
	if s == nil || s.envs == nil {
		return nil
	}
	if id := strings.TrimSpace(s.mqEnvID); id != "" {
		if _, cfg, err := s.envs.DecryptRow(ctx, id); err == nil {
			return cfg
		}
	}
	_, cfg, err := s.envs.FindKind(ctx, "", "mq")
	if err != nil {
		return nil
	}
	return cfg
}

func (s *Service) mqMailReady(ctx context.Context) bool {
	if s == nil || !s.fromMQ || s.envs == nil {
		return false
	}
	raw := s.mqConfig(ctx)
	tpl := configString(raw, "mail_template")
	switch tpl {
	case env.MailTemplateFTPUpdate, env.MailTemplateOSSUpdate, env.MailTemplateProjectUpdate, env.MailTemplateMqNotify:
		return configString(raw, "smtp_env_rec_id") != ""
	default:
		return false
	}
}

// linkedOSSRaw 读取 mq 环境上关联的 OSS。未填写时 ok 为 false，调用方改用已启用的 OSS。
func (s *Service) linkedOSSRaw(ctx context.Context) (map[string]any, bool, error) {
	if s == nil || s.envs == nil {
		return nil, false, nil
	}
	id := configString(s.mqConfig(ctx), "oss_env_rec_id")
	if id == "" {
		return nil, false, nil
	}
	row, raw, err := s.envs.DecryptRow(ctx, id)
	if err != nil {
		return nil, false, fmt.Errorf("linked oss env: %w", err)
	}
	if row.IsEnabled == 0 {
		return nil, false, errors.New("linked oss env is disabled")
	}
	kind, err := env.NormalizeKind(row.EnvKind)
	if err != nil || kind != "oss" {
		return nil, false, errors.New("linked env is not oss")
	}
	if cfg := hostoss.MapConfig(raw); cfg.Endpoint == "" || cfg.Bucket == "" || cfg.AccessKeyID == "" || cfg.AccessKeySecret == "" {
		return nil, false, errors.New("linked oss env is not configured")
	}
	return raw, true, nil
}

// notifyLinked 按关联的 SMTP 和模板发信。推送通知沿用 FTP 已更新版式。构建日志留在平台。
func (s *Service) notifyLinked(ctx context.Context, n Notice, prj project.Project, ref string, ok bool, message, remote, signed, until, backup string) string {
	if s == nil || !s.fromMQ || s.envs == nil {
		return ""
	}
	raw := s.mqConfig(ctx)
	smtpID := configString(raw, "smtp_env_rec_id")
	tpl := configString(raw, "mail_template")
	if tpl == "none" {
		tpl = ""
	}
	if smtpID == "" || tpl == "" {
		return "未发送：消息队列未关联 SMTP 环境与邮件模板"
	}
	switch tpl {
	case env.MailTemplateFTPUpdate, env.MailTemplateOSSUpdate, env.MailTemplateProjectUpdate, env.MailTemplateMqNotify:
	default:
		return "未发送：请选择 FTP 已更新、OSS 已更新、项目已更新或推送通知"
	}
	row, smtpRaw, err := s.envs.DecryptRow(ctx, smtpID)
	if err != nil {
		return "未发送：" + err.Error()
	}
	kind, err := env.NormalizeKind(row.EnvKind)
	if err != nil || kind != "smtp" || row.IsEnabled == 0 {
		return "未发送：关联的 SMTP 环境不可用"
	}
	sc := hostsmtp.MapConfig(smtpRaw)
	to := strings.Join(n.Recipients, ",")
	if strings.TrimSpace(to) == "" {
		to = sc.To
	}
	if strings.TrimSpace(to) == "" {
		return "未发送：未配置收件人"
	}
	sc.To = to
	path := firstNonEmptyPath(remote, n.OSSProjectRoot, n.FTPProjectRoot)
	composed, err := composeLinkedMail(tpl, resourceLabel(n), n.CustomerShortName, backup, env.PackMailFields{
		OK:       ok,
		TaskNo:   n.TaskNo,
		TaskID:   n.TaskID,
		Customer: n.CustomerCode,
		Version:  n.Version,
		Branch:   n.CustomerBranch,
		Project:  prj.ProjectName,
		Result:   message,
		Remote:   path,
		URL:      signed,
		Until:    until,
	})
	if err != nil {
		return "未发送：" + err.Error()
	}
	if sc.FromName == "" {
		sc.FromName = composed.FromName
	}
	if err := hostsmtp.SendMIME(sc, composed.Subject, composed.Text, composed.HTML); err != nil {
		return "未发送：" + err.Error()
	}
	return "已发送 收件人 " + to + "，模板 " + mailTemplateLabel(tpl)
}

func resourceLabel(n Notice) string {
	if len(n.ResourceFiles) == 0 {
		return "全部 war"
	}
	return strings.Join(n.ResourceFiles, ", ")
}

func composeLinkedMail(tpl, resources, projectTitle, backup string, fields env.PackMailFields) (env.ComposedMail, error) {
	switch tpl {
	case env.MailTemplateMqNotify, env.MailTemplateFTPUpdate, env.MailTemplateOSSUpdate:
		resolved := tpl
		if tpl == env.MailTemplateMqNotify {
			resolved = env.MailTemplateFTPUpdate
			if strings.TrimSpace(fields.URL) != "" {
				resolved = env.MailTemplateOSSUpdate
			}
		}
		return env.ComposeMqMail(resolved, env.MqMailFields{
			Project:   projectTitle,
			TaskNo:    fields.TaskNo,
			Resources: resources,
			Remote:    fields.Remote,
			URL:       fields.URL,
			Until:     fields.Until,
			Branch:    fields.Branch,
			Backup:    backup,
		})
	case env.MailTemplateProjectUpdate:
		name := filepath.Base(strings.TrimRight(strings.ReplaceAll(fields.Remote, "\\", "/"), "/"))
		if name == "." || name == string(filepath.Separator) {
			name = ""
		}
		label := strings.TrimSpace(fields.Project)
		if label == "" {
			label = strings.TrimSpace(fields.Customer)
		}
		content := strings.TrimSpace(fields.Remote)
		if content == "" {
			content = "尚未推送"
		} else {
			content = "推送路径 " + content
		}
		if until := strings.TrimSpace(fields.Until); until != "" {
			content += "，有效期至 " + until
		}
		got, err := env.ComposeUpdateMail(tpl, env.UpdateMailFields{
			ProjectLabel:     label,
			UpdateContent:    content,
			TaskNo:           fields.TaskNo,
			FTPVersionSource: fields.Remote,
			UpdateScope:      fields.Branch,
			WarPackage:       name,
			Other:            fields.URL,
		}, time.Now())
		return got, err
	default:
		return env.ComposedMail{}, errors.New("unknown mail template")
	}
}

func mailTemplateLabel(tpl string) string {
	switch tpl {
	case env.MailTemplateFTPUpdate:
		return "FTP 已更新"
	case env.MailTemplateOSSUpdate:
		return "OSS 已更新"
	case env.MailTemplateProjectUpdate:
		return "项目已更新"
	case env.MailTemplateMqNotify:
		return "推送通知"
	default:
		return tpl
	}
}

// receiptSteps 是回执队列里的三步结果。构建日志留在平台，不写进回执。
func receiptSteps(result, message string, n Notice, draft mailDraft, mailNote string) string {
	pack, push := splitPackPush(result, message, n, draft)
	if strings.TrimSpace(mailNote) == "" {
		mailNote = "未发送"
	}
	return "1. 打包：" + pack + "\n2. 推送：" + push + "\n3. 邮件：" + mailNote
}

func splitPackPush(result, message string, n Notice, draft mailDraft) (string, string) {
	msg := strings.TrimSpace(message)
	if msg == msgUpload {
		return "成功", "失败。" + msg
	}
	if result == resultSuccess {
		pack := "成功"
		if msg != "" {
			pack += "。" + msg
		}
		path := firstNonEmptyPath(draft.remote, n.OSSProjectRoot, n.FTPProjectRoot)
		push := "成功"
		if path != "" {
			push += "，" + pushVia(n) + " 路径 " + path
		}
		if files := strings.Join(n.ResourceFiles, ", "); files != "" {
			push += "，资源 " + files
		}
		return pack, push
	}
	if msg == "" {
		msg = "失败"
	}
	return "失败。" + msg, "未执行"
}

func pushVia(n Notice) string {
	oss := strings.TrimSpace(n.OSSProjectRoot) != ""
	ftp := strings.TrimSpace(n.FTPProjectRoot) != ""
	switch {
	case oss && ftp:
		return "OSS、FTP"
	case ftp:
		return "FTP"
	default:
		return "OSS"
	}
}
