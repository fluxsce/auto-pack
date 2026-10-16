package intake

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"auto-pack/server/internal/access"
	"auto-pack/server/internal/audit"
	"auto-pack/server/internal/env"
	"auto-pack/server/internal/iam"
	"auto-pack/server/internal/job"
	"auto-pack/server/internal/mqlog"
	"auto-pack/server/internal/project"
	"auto-pack/server/internal/store"
	"auto-pack/server/internal/workspace"
)

// Jobs 是现有作业入队与等待。接入层不另写执行步骤。
type Jobs interface {
	SubmitSystem(ctx context.Context, req job.SubmitRequest) (job.Job, error)
	Wait(ctx context.Context, id string) (job.Job, []job.Step, error)
}

// Mailer 复用作业结束邮件。上传结束后由接入层再调一次。
type Mailer interface {
	NotifyJob(ctx context.Context, id string, ok bool, message string) error
}

// Auditor 写入操作审计。消息队列触发的每笔通知记一条，插入失败不影响打包。
type Auditor interface {
	Note(ctx context.Context, actor iam.Actor, workspaceID, action, targetType, targetID, summary string, opErr error)
}

// Options 装配接入层。Upload 为空时用平台里的 OSS / FTP 环境，目标路径取报文。
type Options struct {
	Store         *store.Store
	Projects      *project.Service
	Envs          *env.Service
	Jobs          Jobs
	NoticeDir     string
	ReceiptDir    string
	WorkspaceRoot string
	Retries       int
	Mailer        Mailer
	Audit         Auditor
	Upload        func(ctx context.Context, prj project.Project, local, remote string) error
	Sign          func(objectKey string, expire time.Duration) (string, error)
	Publish       func(ctx context.Context, body []byte) error
	Builds        WorkspaceBuild
	MQLog         *mqlog.Log
}

// BuildProject is one project inside a workspace auto-build.
type BuildProject struct {
	Name   string
	Status string
}

// BuildOutcome is one finished workspace auto-build.
type BuildOutcome struct {
	Status    string
	Succeeded int
	Skipped   int
	Failed    int
	Message   string
	Projects  []BuildProject
}

// WorkspaceBuild starts and waits for the workspace auto-build used when a notice names no project.
type WorkspaceBuild interface {
	StartWorkspace(ctx context.Context, workspaceID string) (string, error)
	WaitWorkspace(ctx context.Context, buildID string) (BuildOutcome, error)
}

// Service 处理一笔打包通知。
type Service struct {
	st         *store.Store
	projects   *project.Service
	envs       *env.Service
	jobs       Jobs
	noticeDir  string
	receiptDir string
	wsRoot     string
	retries    int
	mailer     Mailer
	aud        Auditor
	fromMQ     bool
	upload     func(ctx context.Context, prj project.Project, local, remote string) error
	sign       func(objectKey string, expire time.Duration) (string, error)
	publish    func(ctx context.Context, body []byte) error
	builds     WorkspaceBuild
	mq         *MNSQueue
	mqSrc      mqSource
	mqlog      *mqlog.Log
	mqEnvID    string
	stop       context.CancelFunc
}

// New 构造接入层。customerBranch 关联工作空间。未给工程名时走该空间自动构建；给出工程名时只构建该工程。
func New(opt Options) *Service {
	retries := opt.Retries
	if retries <= 0 {
		retries = 3
	}
	s := &Service{
		st:         opt.Store,
		projects:   opt.Projects,
		envs:       opt.Envs,
		jobs:       opt.Jobs,
		noticeDir:  opt.NoticeDir,
		receiptDir: opt.ReceiptDir,
		wsRoot:     opt.WorkspaceRoot,
		retries:    retries,
		mailer:     opt.Mailer,
		aud:        opt.Audit,
		upload:     opt.Upload,
		sign:       opt.Sign,
		publish:    opt.Publish,
		builds:     opt.Builds,
		mqlog:      opt.MQLog,
	}
	if s.publish == nil {
		s.publish = s.publishFile
	}
	return s
}

// Handle 处理一笔通知。同一 taskId 已有记录时不重复打包：已结束的按原结果再写回执并注明原因，尚未结束的只记下原因。
// 校验失败不入队。作业成败仍由现有 runner 决定，上传失败不改作业状态。
func (s *Service) Handle(ctx context.Context, raw []byte) error {
	n, err := parseNotice(raw)
	if err != nil {
		return s.finishWithoutTask(ctx, Notice{}, "报文校验失败：报文无法解析")
	}
	if strings.TrimSpace(n.TaskID) == "" {
		return s.finishWithoutTask(ctx, n, "报文校验失败：taskId 必须非空")
	}
	claimed, err := s.claim(ctx, n, raw)
	if err != nil {
		s.noteMQ(ctx, "", n, "领取打包任务失败", "", false)
		return err
	}
	if !claimed {
		return s.receiptExisting(ctx, n)
	}
	if reason := validateDataHub(n); reason != "" {
		wsID, _ := s.workspaceByCustomerBranch(ctx, n.CustomerBranch)
		return s.finish(ctx, n, project.Project{WorkspaceRecID: wsID}, "", resultFailed, "报文校验失败："+reason)
	}
	wsID, err := s.workspaceByCustomerBranch(ctx, n.CustomerBranch)
	if err != nil {
		return s.finish(ctx, n, project.Project{}, "", resultFailed, "报文校验失败："+err.Error())
	}
	if name := strings.TrimSpace(n.ProjectName); name != "" {
		prj, perr := s.packProject(ctx, wsID, name)
		if perr != nil {
			return s.finish(ctx, n, project.Project{WorkspaceRecID: wsID}, "", resultFailed, "报文校验失败："+perr.Error())
		}
		return s.buildOne(ctx, n, prj)
	}
	return s.buildWorkspace(ctx, n, wsID)
}

func (s *Service) buildOne(ctx context.Context, n Notice, prj project.Project) error {
	recipe := project.ParseRecipe(prj.RecipeJSON)
	req := job.SubmitRequest{
		WorkspaceRecID: prj.WorkspaceRecID,
		ProjectRecID:   prj.RecID,
		JobType:        job.TypeBuild,
		Payload: job.Payload{
			Branch:       n.CustomerBranch,
			TaskRef:      n.TaskID,
			TaskNo:       n.TaskNo,
			CustomerCode: n.CustomerCode,
			Version:      n.Version,
			RemotePath:   firstNonEmptyPath(n.OSSProjectRoot, n.FTPProjectRoot),
			Command:      packCommand(recipe.Command, n.CustomerCode, n.Version, n.SYSVAL),
			MailTo:       strings.Join(n.Recipients, ","),
			HoldMail:     true,
		},
	}
	j, err := s.jobs.SubmitSystem(ctx, req)
	if err != nil {
		log.Printf("intake submit %s: %v", n.TaskID, err)
		return s.finish(ctx, n, prj, "", resultFailed, msgBuild)
	}
	s.rememberRun(ctx, n, prj, j.RecID)
	done, _, err := s.jobs.Wait(ctx, j.RecID)
	if err != nil {
		return s.finish(ctx, n, prj, j.RecID, resultFailed, msgBuild)
	}
	if done.JobStatus != job.StatusSucceeded {
		return s.finish(ctx, n, prj, done.RecID, resultFailed, msgBuild)
	}
	_, remote, signed, until, err := s.pushProjectWars(ctx, prj, n)
	if err != nil {
		log.Printf("intake upload %s: %v", n.TaskID, err)
		s.deliverMail(ctx, done.RecID, false, msgUpload, "", "", "")
		return s.finish(ctx, n, prj, done.RecID, resultFailed, msgUpload)
	}
	s.deliverMail(ctx, done.RecID, true, msgSuccess, remote, signed, until)
	return s.finish(ctx, n, prj, done.RecID, resultSuccess, msgSuccess)
}

func (s *Service) buildWorkspace(ctx context.Context, n Notice, workspaceID string) error {
	anchor := project.Project{WorkspaceRecID: workspaceID}
	pack, perr := s.packProject(ctx, workspaceID, "")
	if perr != nil {
		return s.finish(ctx, n, anchor, "", resultFailed, perr.Error())
	}
	anchor = pack
	if s.builds == nil {
		return s.finish(ctx, n, anchor, "", resultFailed, errNoWorkspaceBuild.Error())
	}
	id, err := s.builds.StartWorkspace(ctx, workspaceID)
	if err != nil {
		log.Printf("intake workspace build %s: %v", n.TaskID, err)
		msg := msgBuild
		if errors.Is(err, errBuildBusy) {
			msg = err.Error()
		}
		return s.finish(ctx, n, anchor, "", resultFailed, msg)
	}
	ref := "build:" + id
	s.rememberRun(ctx, n, anchor, ref)
	out, err := s.builds.WaitWorkspace(ctx, id)
	if err != nil {
		return s.finish(ctx, n, anchor, ref, resultFailed, msgBuild)
	}
	ok, message := finalBuildResult(out, pack.ProjectName)
	if !ok {
		return s.finish(ctx, n, anchor, ref, resultFailed, message)
	}
	if _, _, _, _, err := s.pushProjectWars(ctx, pack, n); err != nil {
		log.Printf("intake upload %s: %v", n.TaskID, err)
		return s.finish(ctx, n, anchor, ref, resultFailed, msgUpload)
	}
	return s.finish(ctx, n, anchor, ref, resultSuccess, message)
}

// finalBuildResult 只看与工作空间编码同名的最终工程。中间工程失败不改变消息队列结果。
func finalBuildResult(out BuildOutcome, finalName string) (bool, string) {
	finalName = strings.TrimSpace(finalName)
	if len(out.Projects) == 0 {
		if out.Status == "success" {
			return true, msgSuccess
		}
		return false, fmt.Sprintf("自动构建未完成：成功 %d，跳过 %d，失败 %d", out.Succeeded, out.Skipped, out.Failed)
	}
	status := ""
	for _, row := range out.Projects {
		if strings.EqualFold(strings.TrimSpace(row.Name), finalName) {
			status = row.Status
		}
	}
	if status == "ok" {
		if out.Failed > 0 {
			return true, fmt.Sprintf("%s。最终工程 %s 构建成功，中间工程失败 %d 个不计入本次结果", msgSuccess, finalName, out.Failed)
		}
		return true, msgSuccess
	}
	if status == "" || status == "skipped" {
		return false, fmt.Sprintf("最终工程 %s 未构建成功", finalName)
	}
	return false, fmt.Sprintf("最终工程 %s 构建失败", finalName)
}

func (s *Service) receiptExisting(ctx context.Context, n Notice) error {
	row, err := s.taskByID(ctx, n.TaskID)
	if err != nil {
		s.noteMQ(ctx, "", n, "读取已有任务失败", "", false)
		return err
	}
	log.Printf("intake receipt existing %s status=%s result=%s", n.TaskID, row.TaskStatus, row.ResultStatus)
	if row.TaskNo != "" {
		n.TaskNo = row.TaskNo
	}
	if row.PackTaskID != "" {
		n.PackTaskID = row.PackTaskID
	}
	if row.OSSProjectRoot != "" {
		n.OSSProjectRoot = row.OSSProjectRoot
		n.FTPProjectRoot = ""
	}
	ws := ""
	if s.projects != nil && row.ProjectRecID != "" {
		if prj, perr := s.projects.GetRaw(ctx, row.ProjectRecID); perr == nil {
			ws = prj.WorkspaceRecID
		}
	}
	ref := s.logRef(ctx, row.JobRecID)
	status := row.ResultStatus
	if status != resultSuccess && status != resultFailed {
		s.noteMQ(ctx, ws, n, "同一 taskId 正在构建，不重复启动。原因：上次构建尚未结束，完成后由原任务回执", ref, true)
		return nil
	}
	message := strings.TrimSpace(row.ResultMessage)
	if message == "" {
		if status == resultSuccess {
			message = msgSuccess
		} else {
			message = "该 taskId 已处理"
		}
	}
	pubErr := s.writeReceipt(ctx, n, status, message)
	text := "同一 taskId 不重复打包。原因：" + message
	if pubErr != nil {
		text += "；回执未写入"
	}
	s.noteMQ(ctx, ws, n, text, ref, pubErr == nil && status == resultSuccess)
	return pubErr
}

func (s *Service) rememberRun(ctx context.Context, n Notice, prj project.Project, ref string) {
	storeID := ref
	if strings.HasPrefix(ref, "build:") {
		storeID = strings.TrimPrefix(ref, "build:")
	}
	if err := s.saveTask(ctx, taskRow{
		TaskID:         n.TaskID,
		ProjectRecID:   prj.RecID,
		JobRecID:       storeID,
		TaskStatus:     statusRunning,
		OSSProjectRoot: firstNonEmptyPath(n.OSSProjectRoot, n.FTPProjectRoot),
	}); err != nil {
		log.Printf("intake remember %s: %v", n.TaskID, err)
	}
	s.noteMQ(ctx, prj.WorkspaceRecID, n, "已接收，开始自动构建", ref, true)
}

func (s *Service) logRef(ctx context.Context, id string) string {
	id = strings.TrimSpace(id)
	if id == "" || strings.HasPrefix(id, "build:") || s.st == nil {
		return id
	}
	q := s.st.Rebind(`SELECT rec_id FROM AP_WORKSPACE_BUILD WHERE rec_id=?`)
	var found string
	if err := s.st.SQL().QueryRowContext(ctx, q, id).Scan(&found); err != nil || found == "" {
		return id
	}
	return "build:" + found
}

func (s *Service) deliverMail(ctx context.Context, jobID string, ok bool, message, remote, url, until string) {
	if s.mailer == nil || strings.TrimSpace(jobID) == "" {
		return
	}
	if err := s.writeMailPayload(ctx, jobID, ok, message, remote, url, until); err != nil {
		log.Printf("intake mail payload %s: %v", jobID, err)
	}
	if err := s.mailer.NotifyJob(ctx, jobID, ok, message); err != nil {
		log.Printf("intake mail %s: %v", jobID, err)
	}
}

func (s *Service) writeMailPayload(ctx context.Context, jobID string, ok bool, message, remote, url, until string) error {
	if s.st == nil {
		return nil
	}
	q := s.st.Rebind(`SELECT payload_json FROM AP_JOB WHERE rec_id = ?`)
	var raw string
	if err := s.st.SQL().QueryRowContext(ctx, q, jobID).Scan(&raw); err != nil {
		return err
	}
	p := job.DecodePayload(raw)
	p.HoldMail = false
	p.MailNote = message
	if remote != "" {
		p.RemotePath = remote
	}
	if ok {
		p.DownloadURL = url
		p.DownloadUntil = until
	}
	body, err := json.Marshal(p)
	if err != nil {
		return err
	}
	upd := s.st.Rebind(`UPDATE AP_JOB SET payload_json = ?, updated_at = ? WHERE rec_id = ?`)
	_, err = s.st.SQL().ExecContext(ctx, upd, string(body), time.Now().UTC().Format("2006-01-02 15:04:05"), jobID)
	return err
}

func (s *Service) projectWork(ctx context.Context, prj project.Project) (string, error) {
	if strings.TrimSpace(s.wsRoot) == "" {
		return "", errTaskDir
	}
	code := s.workspaceCode(ctx, prj.WorkspaceRecID)
	rel := strings.TrimSpace(prj.WorkRelPath)
	if rel == "" {
		rel = project.WorkRel(prj.ProjectName, project.ParseRecipe(prj.RecipeJSON))
	}
	return workspace.ProjectDir(s.wsRoot, code, prj.WorkspaceRecID, rel), nil
}

func (s *Service) workspaceCode(ctx context.Context, id string) string {
	if s.st == nil || strings.TrimSpace(id) == "" {
		return ""
	}
	q := s.st.Rebind(`SELECT workspace_code FROM AP_WORKSPACE WHERE rec_id = ?`)
	var code string
	if err := s.st.SQL().QueryRowContext(ctx, q, id).Scan(&code); err != nil {
		return ""
	}
	return strings.TrimSpace(code)
}

// workspaceByCustomerBranch 按工程 repo_branch = customerBranch 找到唯一工作空间。
func (s *Service) workspaceByCustomerBranch(ctx context.Context, branch string) (string, error) {
	branch = strings.TrimSpace(branch)
	if branch == "" || s.projects == nil {
		return "", errNoBranchWorkspace
	}
	list, err := s.projects.ListEnabledByRepoBranch(ctx, branch)
	if err != nil {
		return "", err
	}
	if len(list) == 0 {
		return "", errNoBranchWorkspace
	}
	id := list[0].WorkspaceRecID
	for _, p := range list[1:] {
		if p.WorkspaceRecID != id {
			return "", errAmbiguousWorkspace
		}
	}
	return id, nil
}

// packProject 决定上传哪个工程。报文给出工程名时用该工程；否则用与工作空间编码同名的工程。
func (s *Service) packProject(ctx context.Context, workspaceID, explicit string) (project.Project, error) {
	if s.projects == nil {
		return project.Project{}, errNoBranchWorkspace
	}
	list, err := s.projects.ListRaw(ctx, workspaceID)
	if err != nil {
		return project.Project{}, err
	}
	if name := strings.TrimSpace(explicit); name != "" {
		return pickNamedProject(list, name)
	}
	code := s.workspaceCode(ctx, workspaceID)
	got, err := pickNamedProject(list, code)
	if err != nil {
		if errors.Is(err, errDisabled) || errors.Is(err, errAmbiguousProject) {
			return project.Project{}, err
		}
		return project.Project{}, errNoPackProject
	}
	return got, nil
}

func pickNamedProject(list []project.Project, name string) (project.Project, error) {
	name = strings.TrimSpace(name)
	var hit project.Project
	enabled, disabled := 0, 0
	for _, p := range list {
		if !strings.EqualFold(strings.TrimSpace(p.ProjectName), name) {
			continue
		}
		if p.IsEnabled == 0 {
			disabled++
			continue
		}
		enabled++
		hit = p
	}
	if enabled == 1 {
		return hit, nil
	}
	if enabled > 1 {
		return project.Project{}, errAmbiguousProject
	}
	if disabled > 0 {
		return project.Project{}, errDisabled
	}
	return project.Project{}, errors.New("未找到打包工程 " + name)
}

func (s *Service) finishWithoutTask(ctx context.Context, n Notice, message string) error {
	err := s.writeReceipt(ctx, n, resultFailed, message)
	text := message
	if err != nil {
		text += "；回执未写入"
	}
	s.noteMQ(ctx, "", n, text, "", false)
	return err
}

func (s *Service) finish(ctx context.Context, n Notice, prj project.Project, jobID, result, message string) error {
	rowStatus := statusFailed
	if result == resultSuccess {
		rowStatus = statusSucceeded
	}
	pubErr := s.writeReceipt(ctx, n, result, message)
	if pubErr != nil {
		rowStatus = statusReceiptFailed
		log.Printf("intake receipt %s: %v", n.TaskID, pubErr)
	}
	finished := time.Now().UTC().Format("2006-01-02 15:04:05")
	storeID := jobID
	if strings.HasPrefix(jobID, "build:") {
		storeID = strings.TrimPrefix(jobID, "build:")
	}
	err := s.saveTask(ctx, taskRow{
		TaskID:         n.TaskID,
		ProjectRecID:   prj.RecID,
		JobRecID:       storeID,
		TaskStatus:     rowStatus,
		ResultStatus:   result,
		ResultMessage:  message,
		OSSProjectRoot: firstNonEmptyPath(n.OSSProjectRoot, n.FTPProjectRoot),
		FinishedAt:     finished,
	})
	text := message
	if pubErr != nil {
		text += "；回执未写入"
	}
	s.noteMQ(ctx, prj.WorkspaceRecID, n, text, jobID, result == resultSuccess && pubErr == nil && err == nil)
	if pubErr != nil && err == nil {
		return nil
	}
	return err
}

func (s *Service) writeReceipt(ctx context.Context, n Notice, result, message string) error {
	body, err := json.Marshal(dataHubReceipt(n, result, message, time.Now()))
	if err != nil {
		return err
	}
	var last error
	for i := 0; i < s.retries; i++ {
		if err := s.publish(ctx, body); err == nil {
			return nil
		} else {
			last = err
		}
		if i+1 >= s.retries {
			break
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Duration(i+1) * 200 * time.Millisecond):
		}
	}
	return last
}

func (s *Service) publishFile(ctx context.Context, body []byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if strings.TrimSpace(s.receiptDir) == "" {
		return errNoReceiptDir
	}
	var probe struct {
		TaskID string `json:"taskId"`
	}
	_ = json.Unmarshal(body, &probe)
	name := sanitizeTask(probe.TaskID) + ".json"
	if err := os.MkdirAll(s.receiptDir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(s.receiptDir, name), body, 0o644)
}

func sanitizeTask(taskID string) string {
	var b strings.Builder
	for _, r := range taskID {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	if b.Len() == 0 {
		return "task"
	}
	return b.String()
}

// handleFromMQ 处理请求队列里的一笔通知，并记操作审计。目录投递不走这里。
func (s *Service) handleFromMQ(ctx context.Context, raw []byte) error {
	if s == nil {
		return nil
	}
	s.fromMQ = true
	defer func() { s.fromMQ = false }()
	return s.Handle(ctx, raw)
}

type auditFail struct{}

func (auditFail) Error() string { return "" }

func (s *Service) noteMQ(ctx context.Context, workspaceID string, n Notice, message, jobID string, ok bool) {
	if s == nil || !s.fromMQ {
		return
	}
	summary := mqSummary(n, message, jobID)
	log.Printf("intake %s", summary)
	if s.aud == nil {
		return
	}
	var opErr error
	if !ok {
		opErr = auditFail{}
	}
	s.aud.Note(ctx, iam.Actor{UserID: "mq"}, access.WorkspaceOrDefault(workspaceID), audit.ActionIntakeMQ, "intake", n.TaskID, summary, opErr)
}

func mqSummary(n Notice, message, jobID string) string {
	var b strings.Builder
	b.WriteString("消息队列")
	if id := strings.TrimSpace(n.TaskID); id != "" {
		b.WriteByte(' ')
		b.WriteString(id)
	}
	if branch := strings.TrimSpace(n.CustomerBranch); branch != "" {
		b.WriteByte(' ')
		b.WriteString(branch)
	}
	if message = strings.TrimSpace(message); message != "" {
		b.WriteString("：")
		b.WriteString(message)
	}
	jobID = strings.TrimSpace(jobID)
	switch {
	case strings.HasPrefix(jobID, "build:"):
		b.WriteString(" build=")
		b.WriteString(strings.TrimPrefix(jobID, "build:"))
	case jobID != "":
		b.WriteString(" job=")
		b.WriteString(jobID)
	}
	return b.String()
}
