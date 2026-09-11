package runner

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	hostsmtp "auto-pack/server/internal/host/smtp"
	"auto-pack/server/internal/job"
	"auto-pack/server/internal/project"
)

func (e *Engine) afterSuccess(ctx context.Context, j job.Job, w io.Writer) {
	// 工作空间批次已按拓扑排队，成功后再按 depends_on 入队会重复构建。
	if j.JobType == job.TypeBuild && !skipDependentEnqueue(j) {
		e.enqueueDependents(ctx, j, w)
	}
}

// skipDependentEnqueue 工作空间批次已按拓扑排队，不再按工程 depends_on 入队下游。
func skipDependentEnqueue(j job.Job) bool {
	switch strings.TrimSpace(job.DecodePayload(j.PayloadJSON).Origin) {
	case job.OriginAutoBuild, job.OriginAutoRelease:
		return true
	default:
		return false
	}
}

func (e *Engine) enqueueDependents(ctx context.Context, j job.Job, w io.Writer) {
	if e.submit == nil || e.projects == nil {
		return
	}
	list, err := e.projects.ListRaw(ctx, j.WorkspaceRecID)
	if err != nil {
		_, _ = fmt.Fprintf(w, "dependents: %v\n", err)
		return
	}
	payload := job.DecodePayload(j.PayloadJSON)
	chain := append(append([]string{}, payload.DependChain...), j.ProjectRecID)
	for _, p := range list {
		if p.RecID == j.ProjectRecID || p.IsEnabled == 0 {
			continue
		}
		dep := project.ParseRecipe(p.RecipeJSON)
		if !project.InDependChain(dep.DependsOn, j.ProjectRecID) {
			continue
		}
		if project.InDependChain(chain, p.RecID) {
			_, _ = fmt.Fprintf(w, "skip dependent %s (cycle)\n", p.ProjectName)
			continue
		}
		_, err := e.submit(ctx, job.SubmitRequest{
			WorkspaceRecID: p.WorkspaceRecID,
			ProjectRecID:   p.RecID,
			JobType:        job.TypeBuild,
			Payload:        job.Payload{DependChain: chain},
		})
		if err != nil {
			_, _ = fmt.Fprintf(w, "enqueue %s: %v\n", p.ProjectName, err)
			continue
		}
		_, _ = fmt.Fprintf(w, "enqueued dependent %s\n", p.ProjectName)
	}
}

func (e *Engine) notify(ctx context.Context, j job.Job, ok bool, errMsg string, w io.Writer) {
	if w == nil {
		w = io.Discard
	}
	if e.st != nil && j.RecID != "" {
		if latest, err := job.JobByID(ctx, e.st, j.RecID); err == nil {
			j = latest
		}
	}
	var prj project.Project
	if j.ProjectRecID != "" && e.projects != nil {
		if p, err := e.projects.GetRaw(ctx, j.ProjectRecID); err == nil {
			prj = p
		}
	}
	e.notifyWithProject(ctx, j, prj, ok, errMsg, w)
}

func (e *Engine) notifyWithProject(ctx context.Context, j job.Job, prj project.Project, ok bool, errMsg string, w io.Writer) {
	if w == nil {
		w = io.Discard
	}
	logLine(w, "== 邮件通知 ==")
	status := "成功"
	if !ok {
		status = "失败"
	}
	sent := 0
	hadEmail := false
	matched := 0
	for _, act := range project.ParseRecipe(prj.RecipeJSON).Actions() {
		if !project.IsEmailAction(act.Kind) {
			continue
		}
		hadEmail = true
		if !project.ActionEnabled(act) {
			logLine(w, "邮件通知：跳过（已停用）")
			continue
		}
		if !project.ActionMatches(act, ok) {
			logLine(w, "邮件通知：跳过（%s，本次作业%s）", postWhenTitle(project.ActionWhen(act)), status)
			continue
		}
		matched++
		if err := e.sendActionMail(ctx, j, prj, act, ok, errMsg, w); err != nil {
			logLine(w, "邮件通知：失败 %v", err)
			continue
		}
		sent++
	}
	if sent > 0 {
		return
	}
	// 工程已配置后置邮件且条件未命中：遵守 when，不再改走空间默认 SMTP。
	if hadEmail && matched == 0 {
		return
	}
	if hadEmail {
		logLine(w, "邮件通知：后置邮件未发出，尝试空间默认 SMTP")
	}
	e.sendWorkspaceMail(ctx, j, prj, ok, errMsg, w)
}

func (e *Engine) sendActionMail(ctx context.Context, j job.Job, prj project.Project, act project.PostAction, ok bool, errMsg string, w io.Writer) error {
	cfg := e.envCfg(ctx, act.EnvRecID, j.WorkspaceRecID, "smtp")
	if len(cfg) == 0 {
		return fmt.Errorf("smtp env not found")
	}
	sc := hostsmtp.MapConfig(cfg)
	if to := strings.TrimSpace(act.To); to != "" {
		sc.To = to
	}
	if len(sc.Recipients()) == 0 {
		logLine(w, "邮件通知：跳过（未配置收件人）")
		return nil
	}
	subject, body := composeNotifyMail(j, prj, e.workspaceTitle(ctx, j.WorkspaceRecID), act, ok, errMsg)
	attach := project.ShouldAttachMailLog(act, ok)
	if attach {
		body = appendMailLog(body, e.logTail(j.RecID, 8000))
	}
	if err := hostsmtp.Send(sc, subject, body); err != nil {
		return err
	}
	logLine(w, "邮件通知：已发送 收件人 %s", strings.Join(sc.Recipients(), ", "))
	logLine(w, "邮件通知：主题 %s", subject)
	if attach {
		logLine(w, "邮件通知：已附带作业日志")
	}
	return nil
}

func (e *Engine) sendWorkspaceMail(ctx context.Context, j job.Job, prj project.Project, ok bool, errMsg string, w io.Writer) {
	cfg := e.kindCfg(ctx, j.WorkspaceRecID, "smtp")
	if len(cfg) == 0 {
		logLine(w, "邮件通知：跳过（空间未配置 SMTP）")
		return
	}
	sc := hostsmtp.MapConfig(cfg)
	if len(sc.Recipients()) == 0 {
		logLine(w, "邮件通知：跳过（未配置收件人）")
		return
	}
	logLine(w, "邮件通知：使用空间默认 SMTP")
	act := project.PostAction{Kind: "email", To: sc.To}
	if err := e.sendActionMail(ctx, j, prj, act, ok, errMsg, w); err != nil {
		logLine(w, "邮件通知：失败 %v", err)
	}
}

func (e *Engine) workspaceTitle(ctx context.Context, id string) string {
	if e.st == nil || strings.TrimSpace(id) == "" {
		return ""
	}
	q := e.st.Rebind(`SELECT workspace_name FROM AP_WORKSPACE WHERE rec_id = ?`)
	var name string
	if err := e.st.SQL().QueryRowContext(ctx, q, id).Scan(&name); err != nil {
		return ""
	}
	return strings.TrimSpace(name)
}

// composeNotifyMail 拼通知主题和正文：工程、分支、状态等关键字段始终写出。
func composeNotifyMail(j job.Job, prj project.Project, workspaceName string, act project.PostAction, ok bool, errMsg string) (string, string) {
	status := job.StatusSucceeded
	statusLabel := "已成功"
	if !ok {
		status = job.StatusFailed
		statusLabel = "失败"
	}
	payload := job.DecodePayload(j.PayloadJSON)
	name := strings.TrimSpace(prj.ProjectName)
	if name == "" {
		name = "-"
	}
	branch := firstNonEmpty(payload.Branch, prj.RepoBranch)
	subject := strings.TrimSpace(act.Subject)
	if subject == "" {
		parts := []string{"[auto-pack]", name}
		if branch != "" {
			parts = append(parts, branch)
		}
		parts = append(parts, mailJobType(j.JobType), statusLabel)
		subject = strings.Join(parts, " ")
	}
	var body strings.Builder
	writeMailField(&body, "工程名称", name)
	writeMailField(&body, "工程类型", dash(prj.ProjectType))
	writeMailField(&body, "工作空间", firstNonEmpty(workspaceName, j.WorkspaceRecID, "-"))
	writeMailField(&body, "分支", dash(branch))
	writeMailField(&body, "仓库", dash(firstNonEmpty(payload.CloneURL, prj.RepoURL)))
	writeMailField(&body, "作业类型", mailJobType(j.JobType))
	writeMailField(&body, "构建状态", statusLabel+" ("+status+")")
	if tag := strings.TrimSpace(payload.Tag); tag != "" {
		writeMailField(&body, "Tag", tag)
	}
	writeMailField(&body, "开始时间", formatMailTime(j.StartedAt))
	writeMailField(&body, "结束时间", formatMailTime(firstNonEmpty(j.FinishedAt, job.NowUTC())))
	writeMailField(&body, "作业编号", dash(j.RecID))
	if errMsg != "" {
		writeMailField(&body, "错误信息", errMsg)
	}
	return subject, body.String()
}

func writeMailField(b *strings.Builder, label, value string) {
	fmt.Fprintf(b, "%s：%s\n", label, value)
}

func mailJobType(code string) string {
	switch code {
	case job.TypeBuild:
		return "构建"
	case job.TypeRelease:
		return "发版"
	case job.TypeFTPPush:
		return "FTP 推送"
	case job.TypePOMVersion:
		return "改 POM"
	case job.TypeBranchClone:
		return "克隆分支"
	case job.TypeSyncSources:
		return "同步源码"
	default:
		return dash(code)
	}
}

func formatMailTime(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "-"
	}
	ts, err := time.ParseInLocation("2006-01-02 15:04:05", raw, time.UTC)
	if err != nil {
		return raw
	}
	return ts.In(time.FixedZone("CST", 8*3600)).Format("2006-01-02 15:04:05")
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if s := strings.TrimSpace(v); s != "" {
			return s
		}
	}
	return ""
}

func dash(s string) string {
	if strings.TrimSpace(s) == "" {
		return "-"
	}
	return strings.TrimSpace(s)
}

func appendMailLog(body, tail string) string {
	tail = strings.TrimSpace(tail)
	if tail == "" {
		return body
	}
	body += "\n--- 构建日志 ---\n" + tail
	if !strings.HasSuffix(body, "\n") {
		body += "\n"
	}
	return body
}

func (e *Engine) logTail(jobID string, max int) string {
	if max <= 0 {
		return ""
	}
	text, _, _, err := e.ReadLog(jobID, 0)
	if err != nil || text == "" {
		return ""
	}
	if len(text) > max {
		return text[len(text)-max:]
	}
	return text
}
