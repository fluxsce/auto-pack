package intake

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"auto-pack/server/internal/access"
	"auto-pack/server/internal/audit"
	"auto-pack/server/internal/iam"
	"auto-pack/server/internal/job"
	"auto-pack/server/internal/migrate"
	"auto-pack/server/internal/project"
	"auto-pack/server/internal/store"
	"auto-pack/server/internal/workspace"
)

type fakeJobs struct {
	status    string
	submitted []job.SubmitRequest
}

func (f *fakeJobs) SubmitSystem(_ context.Context, req job.SubmitRequest) (job.Job, error) {
	f.submitted = append(f.submitted, req)
	return job.Job{RecID: "job-1", JobStatus: job.StatusQueued}, nil
}

func (f *fakeJobs) Wait(_ context.Context, id string) (job.Job, []job.Step, error) {
	return job.Job{RecID: id, JobStatus: f.status}, nil, nil
}

type fakeBuild struct {
	status   string
	starts   int
	startErr error
	failed   int
	projects []BuildProject
}

func (f *fakeBuild) StartWorkspace(context.Context, string) (string, error) {
	f.starts++
	if f.startErr != nil {
		return "", f.startErr
	}
	return "build-1", nil
}

func (f *fakeBuild) WaitWorkspace(context.Context, string) (BuildOutcome, error) {
	st := f.status
	if st == "" {
		st = "success"
	}
	out := BuildOutcome{Status: st, Succeeded: 1, Failed: f.failed, Projects: f.projects}
	if st != "success" && f.failed == 0 && len(f.projects) == 0 {
		out.Succeeded = 0
		out.Failed = 1
	}
	return out, nil
}

func TestHandleValidationDoesNotSubmit(t *testing.T) {
	svc, jobs, receipts, mails := setupIntake(t)
	n := sampleNotice()
	n.PackagingMode = "jar"
	raw, _ := json.Marshal(noticeJSON(n))
	if err := svc.Handle(context.Background(), raw); err != nil {
		t.Fatal(err)
	}
	if len(jobs.submitted) != 0 || mails.n != 0 {
		t.Fatalf("submitted %d mails %d", len(jobs.submitted), mails.n)
	}
	got := decodeReceipt(t, (*receipts)[0])
	if got.ResultStatus != resultFailed || got.OSSProjectRoot != n.OSSProjectRoot {
		t.Fatalf("%+v", got)
	}
	if got.SubsystemType != subsystemDataHub || got.PackagingMode != "war" {
		t.Fatalf("fixed fields %+v", got)
	}
}

func TestHandleDuplicateIgnored(t *testing.T) {
	svc, jobs, receipts, _ := setupIntake(t)
	jobs.status = job.StatusSucceeded
	raw := mustNotice(sampleNotice())
	if err := svc.Handle(context.Background(), raw); err != nil {
		t.Fatal(err)
	}
	if err := svc.Handle(context.Background(), raw); err != nil {
		t.Fatal(err)
	}
	fb := svc.builds.(*fakeBuild)
	if fb.starts != 1 || len(jobs.submitted) != 0 || len(*receipts) != 2 {
		t.Fatalf("starts=%d submit=%d receipt=%d", fb.starts, len(jobs.submitted), len(*receipts))
	}
	again := decodeReceipt(t, (*receipts)[1])
	if again.ResultStatus != resultSuccess || again.TaskID != sampleNotice().TaskID || again.ResultMessage != msgSuccess {
		t.Fatalf("%+v", again)
	}
}

func TestDuplicateWhileRunningKeepsReason(t *testing.T) {
	svc, jobs, receipts, _ := setupIntake(t)
	ctx := context.Background()
	n := sampleNotice()
	if _, err := svc.st.SQL().ExecContext(ctx, `INSERT INTO AP_WORKSPACE_BUILD (rec_id, workspace_rec_id, build_num, status)
		VALUES ('build-1', 'ws-1', 1, 'running')`); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Format("2006-01-02 15:04:05")
	if _, err := svc.st.SQL().ExecContext(ctx, `INSERT INTO AP_INTAKE_TASK (
		task_id, task_no, pack_task_id, subsystem_type, project_rec_id, job_rec_id,
		task_status, result_status, result_message, oss_project_root, notice_json, finished_at, created_at, updated_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		n.TaskID, n.TaskNo, n.PackTaskID, n.SubsystemType, "p-code", "build-1",
		statusRunning, "", "", n.OSSProjectRoot, "{}", nil, now, now); err != nil {
		t.Fatal(err)
	}
	aud := &fakeAudit{}
	svc.aud = aud
	if err := svc.handleFromMQ(ctx, mustNotice(n)); err != nil {
		t.Fatal(err)
	}
	fb := svc.builds.(*fakeBuild)
	if fb.starts != 0 || len(jobs.submitted) != 0 || len(*receipts) != 0 {
		t.Fatalf("starts=%d submit=%d receipt=%d", fb.starts, len(jobs.submitted), len(*receipts))
	}
	if len(aud.calls) != 1 {
		t.Fatalf("calls=%d", len(aud.calls))
	}
	got := aud.calls[0].summary
	if !strings.Contains(got, "正在构建") || !strings.Contains(got, "原因：") || !strings.Contains(got, "build=build-1") {
		t.Fatalf("%s", got)
	}
}

func TestDuplicateAfterFinishStatesReason(t *testing.T) {
	svc, _, _, _ := setupIntake(t)
	ctx := context.Background()
	raw := mustNotice(sampleNotice())
	aud := &fakeAudit{}
	svc.aud = aud
	if err := svc.handleFromMQ(ctx, raw); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.st.SQL().ExecContext(ctx, `INSERT INTO AP_WORKSPACE_BUILD (rec_id, workspace_rec_id, build_num, status)
		VALUES ('build-1', 'ws-1', 1, 'success')`); err != nil {
		t.Fatal(err)
	}
	if err := svc.handleFromMQ(ctx, raw); err != nil {
		t.Fatal(err)
	}
	if len(aud.calls) != 3 {
		t.Fatalf("calls=%d", len(aud.calls))
	}
	got := aud.calls[2].summary
	if !aud.calls[2].ok || !strings.Contains(got, "不重复打包") || !strings.Contains(got, "原因："+msgSuccess) || !strings.Contains(got, "build=build-1") {
		t.Fatalf("%s", got)
	}
}

func TestHandleBuildFailureSkipsUpload(t *testing.T) {
	svc, jobs, receipts, mails := setupIntake(t)
	jobs.status = job.StatusFailed
	n := sampleNotice()
	n.ProjectName = "datahub"
	uploads := 0
	svc.upload = func(context.Context, project.Project, string, string) error {
		uploads++
		return nil
	}
	if err := svc.Handle(context.Background(), mustNotice(n)); err != nil {
		t.Fatal(err)
	}
	if uploads != 0 {
		t.Fatalf("uploads=%d", uploads)
	}
	if jobs.submitted[0].Payload.MailTo != "a@b.c,c@d.e" || !jobs.submitted[0].Payload.HoldMail {
		t.Fatalf("%+v", jobs.submitted[0].Payload)
	}
	if mails.n != 0 {
		t.Fatalf("build failure mail is sent by the job, got %d", mails.n)
	}
	if decodeReceipt(t, (*receipts)[0]).ResultStatus != resultFailed {
		t.Fatal(string((*receipts)[0]))
	}
	row, err := svc.taskByID(context.Background(), sampleNotice().TaskID)
	if err != nil {
		t.Fatal(err)
	}
	if row.TaskStatus != statusFailed || row.JobRecID != "job-1" {
		t.Fatalf("%+v", row)
	}
}

func TestHandleUploadFailureKeepsJobSuccess(t *testing.T) {
	svc, jobs, receipts, mails := setupIntake(t)
	jobs.status = job.StatusSucceeded
	n := sampleNotice()
	n.ProjectName = "datahub"
	svc.upload = func(context.Context, project.Project, string, string) error {
		return errNoWar
	}
	if err := svc.Handle(context.Background(), mustNotice(n)); err != nil {
		t.Fatal(err)
	}
	got := decodeReceipt(t, (*receipts)[0])
	if got.ResultStatus != resultFailed || got.OSSProjectRoot != "/开发/S01/" {
		t.Fatalf("%+v", got)
	}
	row, err := svc.taskByID(context.Background(), sampleNotice().TaskID)
	if err != nil {
		t.Fatal(err)
	}
	if row.ResultStatus != resultFailed || row.TaskStatus != statusFailed {
		t.Fatalf("%+v", row)
	}
	if mails.n != 1 || mails.ok || mails.msg != msgUpload {
		t.Fatalf("mail n=%d ok=%v msg=%s", mails.n, mails.ok, mails.msg)
	}
	payload := jobPayload(t, svc, "job-1")
	if payload.DownloadURL != "" || payload.HoldMail || payload.MailNote != msgUpload {
		t.Fatalf("%+v", payload)
	}
}

func TestHandleSuccess(t *testing.T) {
	svc, jobs, receipts, mails := setupIntake(t)
	jobs.status = job.StatusSucceeded
	n := sampleNotice()
	n.ProjectName = "datahub"
	var remote, uploaded string
	svc.upload = func(_ context.Context, prj project.Project, local, dest string) error {
		uploaded = prj.ProjectName
		remote = dest
		if filepath.Base(local) != "app.war" {
			t.Fatalf("local %s", local)
		}
		return nil
	}
	if err := svc.Handle(context.Background(), mustNotice(n)); err != nil {
		t.Fatal(err)
	}
	if remote != "/开发/S01/app.war" || uploaded != "datahub" || svc.builds.(*fakeBuild).starts != 0 {
		t.Fatalf("remote=%s project=%s starts=%d", remote, uploaded, svc.builds.(*fakeBuild).starts)
	}
	req := jobs.submitted[0]
	if req.Payload.SkipSync || req.Payload.CloneDir != "" || req.Payload.Branch != "release/customer" || req.Payload.TaskRef != sampleNotice().TaskID {
		t.Fatalf("%+v", req.Payload)
	}
	if req.Payload.Origin != "" || req.WorkspaceRecID != "ws-1" || req.Payload.MailTo != "a@b.c,c@d.e" || !req.Payload.HoldMail {
		t.Fatalf("origin=%q ws=%q to=%q", req.Payload.Origin, req.WorkspaceRecID, req.Payload.MailTo)
	}
	if !strings.Contains(req.Payload.Command, "SYSVAL") || !strings.Contains(req.Payload.Command, "V10DATAHUB") || !strings.Contains(req.Payload.Command, "customerCode") || !strings.Contains(req.Payload.Command, "C01") {
		t.Fatal(req.Payload.Command)
	}
	got := decodeReceipt(t, (*receipts)[0])
	if got.ResultStatus != resultSuccess || got.ResultMessage != msgSuccess {
		t.Fatalf("%+v", got)
	}
	if mails.n != 1 || !mails.ok || mails.msg != msgSuccess {
		t.Fatalf("mail n=%d ok=%v msg=%s", mails.n, mails.ok, mails.msg)
	}
	payload := jobPayload(t, svc, "job-1")
	if payload.HoldMail || payload.RemotePath != "/开发/S01/app.war" || payload.MailNote != msgSuccess {
		t.Fatalf("%+v", payload)
	}
	if !strings.Contains(payload.DownloadURL, "开发/S01/app.war") || !strings.Contains(payload.DownloadURL, "Signature=") || payload.DownloadUntil == "" {
		t.Fatalf("%+v", payload)
	}
}

func TestHandleReceiptFailureDoesNotRewriteSuccess(t *testing.T) {
	svc, jobs, _, _ := setupIntake(t)
	jobs.status = job.StatusSucceeded
	svc.retries = 1
	svc.publish = func(context.Context, []byte) error { return errNoReceiptDir }
	svc.upload = func(context.Context, project.Project, string, string) error { return nil }
	if err := svc.Handle(context.Background(), mustNotice(sampleNotice())); err != nil {
		t.Fatal(err)
	}
	row, err := svc.taskByID(context.Background(), sampleNotice().TaskID)
	if err != nil {
		t.Fatal(err)
	}
	if row.TaskStatus != statusReceiptFailed || row.ResultStatus != resultSuccess {
		t.Fatalf("%+v", row)
	}
}

type recMail struct {
	n   int
	ok  bool
	msg string
}

func (m *recMail) NotifyJob(_ context.Context, _ string, ok bool, message string) error {
	m.n++
	m.ok = ok
	m.msg = message
	return nil
}

func setupIntake(t *testing.T) (*Service, *fakeJobs, *[][]byte, *recMail) {
	t.Helper()
	ctx := context.Background()
	st, err := store.Open(ctx, store.Config{Driver: "sqlite", Path: filepath.Join(t.TempDir(), "t.db")})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if _, err := migrate.Apply(ctx, st, "sqlite"); err != nil {
		t.Fatal(err)
	}
	if _, err := st.SQL().ExecContext(ctx, `INSERT INTO AP_WORKSPACE (rec_id, workspace_code, workspace_name, owner_user_id)
		VALUES ('ws-1', 'dh', 'DataHub', 'u1')`); err != nil {
		t.Fatal(err)
	}
	recipe := `{"command":"mvn -B package","output_dir":"target","artifact_pattern":"*.war"}`
	q := `INSERT INTO AP_PROJECT (rec_id, workspace_rec_id, project_name, project_type, repo_url, repo_branch, recipe_json, is_enabled)
		VALUES ('p-dh', 'ws-1', 'datahub', 'maven', 'https://git.example/dh.git', 'release/customer', ?, 1)`
	if _, err := st.SQL().ExecContext(ctx, q, recipe); err != nil {
		t.Fatal(err)
	}
	if _, err := st.SQL().ExecContext(ctx, `INSERT INTO AP_JOB (rec_id, workspace_rec_id, project_rec_id, job_type, job_status, payload_json, created_by)
		VALUES ('job-1', 'ws-1', 'p-dh', 'build', 'succeeded', '{}', 'system')`); err != nil {
		t.Fatal(err)
	}
	jobs := &fakeJobs{status: job.StatusSucceeded}
	var receipts [][]byte
	mails := &recMail{}
	root := t.TempDir()
	warDir := filepath.Join(workspace.ProjectDir(root, "dh", "ws-1", "datahub"), "target")
	if err := os.MkdirAll(warDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(warDir, "app.war"), []byte("war"), 0o644); err != nil {
		t.Fatal(err)
	}
	codeQ := `INSERT INTO AP_PROJECT (rec_id, workspace_rec_id, project_name, project_type, repo_url, repo_branch, recipe_json, is_enabled)
		VALUES ('p-code', 'ws-1', 'dh', 'maven', 'https://git.example/code.git', 'release/customer', ?, 1)`
	if _, err := st.SQL().ExecContext(ctx, codeQ, recipe); err != nil {
		t.Fatal(err)
	}
	codeWar := filepath.Join(workspace.ProjectDir(root, "dh", "ws-1", "dh"), "target")
	if err := os.MkdirAll(codeWar, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(codeWar, "app.war"), []byte("pack"), 0o644); err != nil {
		t.Fatal(err)
	}
	svc := New(Options{
		Store:         st,
		Projects:      project.NewService(st, nil),
		Jobs:          jobs,
		WorkspaceRoot: root,
		Retries:       1,
		Mailer:        mails,
		Upload:        func(context.Context, project.Project, string, string) error { return nil },
		Sign: func(key string, expire time.Duration) (string, error) {
			if key != "开发/S01/app.war" || expire != 24*time.Hour {
				return "", errNoWar
			}
			return "https://datahub.oss-cn-hangzhou.aliyuncs.com/" + key + "?Expires=1&Signature=x", nil
		},
		Publish: func(_ context.Context, body []byte) error {
			receipts = append(receipts, append([]byte(nil), body...))
			return nil
		},
	})
	svc.builds = &fakeBuild{}
	return svc, jobs, &receipts, mails
}

func TestPickNamedProject(t *testing.T) {
	list := []project.Project{
		{RecID: "web", ProjectName: "flux-collaboration-datahub01web", ProjectType: "maven", IsEnabled: 1},
		{RecID: "dh", ProjectName: "flux-collaboration-datahub", ProjectType: "maven", IsEnabled: 1},
		{RecID: "off", ProjectName: "legacy", IsEnabled: 0},
	}
	got, err := pickNamedProject(list, "flux-collaboration-datahub")
	if err != nil || got.RecID != "dh" {
		t.Fatalf("%+v %v", got, err)
	}
	if _, err := pickNamedProject(list, "missing"); err == nil || !strings.Contains(err.Error(), "未找到打包工程") {
		t.Fatalf("err=%v", err)
	}
	if _, err := pickNamedProject(list, "legacy"); err != errDisabled {
		t.Fatalf("err=%v", err)
	}
}

func TestIntermediateFailureDoesNotFailReceipt(t *testing.T) {
	svc, _, receipts, _ := setupIntake(t)
	svc.builds = &fakeBuild{
		status: "partial",
		failed: 1,
		projects: []BuildProject{
			{Name: "datahub", Status: "failed"},
			{Name: "dh", Status: "ok"},
		},
	}
	if err := svc.Handle(context.Background(), mustNotice(sampleNotice())); err != nil {
		t.Fatal(err)
	}
	got := decodeReceipt(t, (*receipts)[0])
	if got.ResultStatus != resultSuccess || !strings.Contains(got.ResultMessage, "最终工程 dh 构建成功") || !strings.Contains(got.ResultMessage, "不计入本次结果") {
		t.Fatalf("%+v", got)
	}
}

func TestFinalProjectFailureFailsReceipt(t *testing.T) {
	svc, _, receipts, _ := setupIntake(t)
	uploads := 0
	svc.upload = func(context.Context, project.Project, string, string) error {
		uploads++
		return nil
	}
	svc.builds = &fakeBuild{
		status: "partial",
		failed: 1,
		projects: []BuildProject{
			{Name: "datahub", Status: "ok"},
			{Name: "dh", Status: "failed"},
		},
	}
	if err := svc.Handle(context.Background(), mustNotice(sampleNotice())); err != nil {
		t.Fatal(err)
	}
	got := decodeReceipt(t, (*receipts)[0])
	if uploads != 0 || got.ResultStatus != resultFailed || !strings.Contains(got.ResultMessage, "最终工程 dh 构建失败") {
		t.Fatalf("uploads=%d %+v", uploads, got)
	}
}

func TestHandleUnnamedStartsWorkspaceBuild(t *testing.T) {
	svc, jobs, receipts, _ := setupIntake(t)
	q := `INSERT INTO AP_PROJECT (rec_id, workspace_rec_id, project_name, project_type, repo_url, repo_branch, recipe_json, is_enabled)
		VALUES ('p-web', 'ws-1', 'flux-collaboration-datahub01web', 'maven', 'https://git.example/web.git', 'release/customer', '{}', 1)`
	if _, err := svc.st.SQL().ExecContext(context.Background(), q); err != nil {
		t.Fatal(err)
	}
	var uploaded []string
	svc.upload = func(_ context.Context, prj project.Project, _, _ string) error {
		uploaded = append(uploaded, prj.ProjectName)
		return nil
	}
	if err := svc.Handle(context.Background(), mustNotice(sampleNotice())); err != nil {
		t.Fatal(err)
	}
	fb := svc.builds.(*fakeBuild)
	if fb.starts != 1 || len(jobs.submitted) != 0 {
		t.Fatalf("starts=%d submitted=%d", fb.starts, len(jobs.submitted))
	}
	if len(uploaded) != 1 || uploaded[0] != "dh" {
		t.Fatalf("uploaded=%v", uploaded)
	}
	got := decodeReceipt(t, (*receipts)[0])
	if got.ResultStatus != resultSuccess {
		t.Fatalf("%+v", got)
	}
	row, err := svc.taskByID(context.Background(), sampleNotice().TaskID)
	if err != nil || row.JobRecID != "build-1" || row.ProjectRecID != "p-code" {
		t.Fatalf("%+v %v", row, err)
	}
}

func TestHandleUnnamedRequiresCodeProject(t *testing.T) {
	svc, jobs, receipts, _ := setupIntake(t)
	if _, err := svc.st.SQL().ExecContext(context.Background(), `DELETE FROM AP_PROJECT WHERE rec_id = 'p-code'`); err != nil {
		t.Fatal(err)
	}
	if err := svc.Handle(context.Background(), mustNotice(sampleNotice())); err != nil {
		t.Fatal(err)
	}
	if svc.builds.(*fakeBuild).starts != 0 || len(jobs.submitted) != 0 {
		t.Fatal("missing pack project must not build")
	}
	got := decodeReceipt(t, (*receipts)[0])
	if got.ResultStatus != resultFailed || !strings.Contains(got.ResultMessage, "工作空间编码") {
		t.Fatalf("%+v", got)
	}
}

func TestHandleUnknownCustomerBranch(t *testing.T) {
	svc, jobs, receipts, _ := setupIntake(t)
	n := sampleNotice()
	n.CustomerBranch = "release/missing"
	if err := svc.Handle(context.Background(), mustNotice(n)); err != nil {
		t.Fatal(err)
	}
	if len(jobs.submitted) != 0 {
		t.Fatal("must not submit")
	}
	got := decodeReceipt(t, (*receipts)[0])
	if got.ResultStatus != resultFailed || !strings.Contains(got.ResultMessage, "customerBranch") {
		t.Fatalf("%+v", got)
	}
}

func jobPayload(t *testing.T, svc *Service, id string) job.Payload {
	t.Helper()
	var raw string
	q := svc.st.Rebind(`SELECT payload_json FROM AP_JOB WHERE rec_id = ?`)
	if err := svc.st.SQL().QueryRowContext(context.Background(), q, id).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	return job.DecodePayload(raw)
}

func mustNotice(n Notice) []byte {
	b, err := json.Marshal(noticeJSON(n))
	if err != nil {
		panic(err)
	}
	return b
}

func noticeJSON(n Notice) map[string]string {
	out := map[string]string{
		"taskId":             n.TaskID,
		"taskNo":             n.TaskNo,
		"packTaskId":         n.PackTaskID,
		"subsystemType":      n.SubsystemType,
		"packagingMode":      n.PackagingMode,
		"deploymentStrategy": n.DeploymentStrategy,
		"ossProjectRoot":     n.OSSProjectRoot,
		"customerCode":       n.CustomerCode,
		"version":            n.Version,
		"customerBranch":     n.CustomerBranch,
		"baseRef":            n.BaseRef,
		"baseRefType":        n.BaseRefType,
		"SYSVAL":             n.SYSVAL,
		"mailTo":             strings.Join(n.Recipients, ","),
		"ftpProjectRoot":     n.FTPProjectRoot,
	}
	if n.ProjectName != "" {
		out["projectName"] = n.ProjectName
	}
	return out
}

func decodeReceipt(t *testing.T, raw []byte) Receipt {
	t.Helper()
	var got Receipt
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	return got
}

type fakeAudit struct {
	calls []fakeAuditCall
}

type fakeAuditCall struct {
	ws, action, targetType, targetID, summary string
	ok                                        bool
}

func (f *fakeAudit) Note(_ context.Context, _ iam.Actor, ws, action, targetType, targetID, summary string, opErr error) {
	f.calls = append(f.calls, fakeAuditCall{ws, action, targetType, targetID, summary, opErr == nil})
}

func TestHandleFromMQWritesAudit(t *testing.T) {
	svc, jobs, _, _ := setupIntake(t)
	jobs.status = job.StatusSucceeded
	aud := &fakeAudit{}
	svc.aud = aud
	if err := svc.handleFromMQ(context.Background(), mustNotice(sampleNotice())); err != nil {
		t.Fatal(err)
	}
	if len(aud.calls) != 2 {
		t.Fatalf("calls=%d", len(aud.calls))
	}
	if !strings.Contains(aud.calls[0].summary, "已接收，开始自动构建") {
		t.Fatalf("start %s", aud.calls[0].summary)
	}
	got := aud.calls[1]
	if !got.ok || got.ws != "ws-1" || got.action != audit.ActionIntakeMQ || got.targetType != "intake" || got.targetID != sampleNotice().TaskID {
		t.Fatalf("%+v", got)
	}
	if !strings.Contains(got.summary, "消息队列") || !strings.Contains(got.summary, "release/customer") || !strings.Contains(got.summary, msgSuccess) || !strings.Contains(got.summary, "build=build-1") {
		t.Fatalf("%s", got.summary)
	}
	if err := svc.Handle(context.Background(), mustNotice(sampleNotice())); err != nil {
		t.Fatal(err)
	}
	if len(aud.calls) != 2 {
		t.Fatal("direct handle must not audit")
	}
}

func TestHandleFromMQAuditsBadPayload(t *testing.T) {
	svc, _, _, _ := setupIntake(t)
	aud := &fakeAudit{}
	svc.aud = aud
	if err := svc.handleFromMQ(context.Background(), []byte(`not-json`)); err != nil {
		t.Fatal(err)
	}
	if len(aud.calls) != 1 || aud.calls[0].ok || aud.calls[0].ws != access.DefaultWorkspaceRecID {
		t.Fatalf("%+v", aud.calls)
	}
	if !strings.Contains(aud.calls[0].summary, "报文无法解析") {
		t.Fatalf("%s", aud.calls[0].summary)
	}
}
