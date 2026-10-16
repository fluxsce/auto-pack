package intake

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"auto-pack/server/internal/env"
	"auto-pack/server/internal/job"
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
	Upload        func(ctx context.Context, prj project.Project, local, remote string) error
	Sign          func(objectKey string, expire time.Duration) (string, error)
	Publish       func(ctx context.Context, body []byte) error
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
	upload     func(ctx context.Context, prj project.Project, local, remote string) error
	sign       func(objectKey string, expire time.Duration) (string, error)
	publish    func(ctx context.Context, body []byte) error
	mq         *MNSQueue
	stop       context.CancelFunc
}

// New 构造接入层。打包工程按报文 customerBranch 关联工作空间，不在 mq 环境里写死。
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
		upload:     opt.Upload,
		sign:       opt.Sign,
		publish:    opt.Publish,
	}
	if s.publish == nil {
		s.publish = s.publishFile
	}
	return s
}

// Handle 处理一笔通知。同一 taskId 已有记录时不重复打包，按已有结果再写回执。
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
		return err
	}
	if !claimed {
		return s.receiptExisting(ctx, n)
	}
	if reason := validateDataHub(n); reason != "" {
		prj, _ := s.projectByCustomerBranch(ctx, n.CustomerBranch)
		return s.finish(ctx, n, prj, "", resultFailed, "报文校验失败："+reason)
	}
	prj, err := s.projectByCustomerBranch(ctx, n.CustomerBranch)
	if err != nil {
		return s.finish(ctx, n, project.Project{}, "", resultFailed, "报文校验失败："+err.Error())
	}
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
	done, _, err := s.jobs.Wait(ctx, j.RecID)
	if err != nil {
		return s.finish(ctx, n, prj, j.RecID, resultFailed, msgBuild)
	}
	if done.JobStatus != job.StatusSucceeded {
		return s.finish(ctx, n, prj, done.RecID, resultFailed, msgBuild)
	}
	work, err := s.projectWork(ctx, prj)
	if err != nil {
		log.Printf("intake workdir %s: %v", n.TaskID, err)
		return s.finish(ctx, n, prj, done.RecID, resultFailed, msgUpload)
	}
	_, remote, signed, until, err := s.pushWar(ctx, prj, work, n)
	if err != nil {
		log.Printf("intake upload %s: %v", n.TaskID, err)
		s.deliverMail(ctx, done.RecID, false, msgUpload, "", "", "")
		return s.finish(ctx, n, prj, done.RecID, resultFailed, msgUpload)
	}
	s.deliverMail(ctx, done.RecID, true, msgSuccess, remote, signed, until)
	return s.finish(ctx, n, prj, done.RecID, resultSuccess, msgSuccess)
}

func (s *Service) receiptExisting(ctx context.Context, n Notice) error {
	row, err := s.taskByID(ctx, n.TaskID)
	if err != nil {
		return err
	}
	log.Printf("intake receipt existing %s status=%s result=%s", n.TaskID, row.TaskStatus, row.ResultStatus)
	status := row.ResultStatus
	message := strings.TrimSpace(row.ResultMessage)
	if status != resultSuccess && status != resultFailed {
		status = resultFailed
		if message == "" {
			message = "该 taskId 已在处理，不重复打包"
		}
	} else if message == "" {
		if status == resultSuccess {
			message = msgSuccess
		} else {
			message = "该 taskId 已处理，不重复打包"
		}
	}
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
	return s.writeReceipt(ctx, n, status, message)
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

// projectByCustomerBranch 按工程 repo_branch = customerBranch 找到唯一工作空间，再选定打包工程。
// 同空间多工程时优先工程名 datahub（忽略大小写），否则唯一 maven，再否则报错。
func (s *Service) projectByCustomerBranch(ctx context.Context, branch string) (project.Project, error) {
	branch = strings.TrimSpace(branch)
	if branch == "" || s.projects == nil {
		return project.Project{}, errNoBranchWorkspace
	}
	list, err := s.projects.ListEnabledByRepoBranch(ctx, branch)
	if err != nil {
		return project.Project{}, err
	}
	if len(list) == 0 {
		return project.Project{}, errNoBranchWorkspace
	}
	ws := map[string]struct{}{}
	for _, p := range list {
		ws[p.WorkspaceRecID] = struct{}{}
	}
	if len(ws) > 1 {
		return project.Project{}, errAmbiguousWorkspace
	}
	return pickDataHubProject(list)
}

func pickDataHubProject(list []project.Project) (project.Project, error) {
	if len(list) == 0 {
		return project.Project{}, errNoBranchWorkspace
	}
	if len(list) == 1 {
		if list[0].IsEnabled == 0 {
			return project.Project{}, errDisabled
		}
		return list[0], nil
	}
	var named []project.Project
	var maven []project.Project
	for _, p := range list {
		if strings.EqualFold(strings.TrimSpace(p.ProjectName), "datahub") {
			named = append(named, p)
		}
		if strings.EqualFold(strings.TrimSpace(p.ProjectType), "maven") {
			maven = append(maven, p)
		}
	}
	if len(named) == 1 {
		return named[0], nil
	}
	if len(maven) == 1 {
		return maven[0], nil
	}
	return project.Project{}, errAmbiguousProject
}

func (s *Service) finishWithoutTask(ctx context.Context, n Notice, message string) error {
	return s.writeReceipt(ctx, n, resultFailed, message)
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
	err := s.saveTask(ctx, taskRow{
		TaskID:         n.TaskID,
		ProjectRecID:   prj.RecID,
		JobRecID:       jobID,
		TaskStatus:     rowStatus,
		ResultStatus:   result,
		ResultMessage:  message,
		OSSProjectRoot: firstNonEmptyPath(n.OSSProjectRoot, n.FTPProjectRoot),
		FinishedAt:     finished,
	})
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
