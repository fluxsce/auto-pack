package intake

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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
	if len(jobs.submitted) != 1 || len(*receipts) != 2 {
		t.Fatalf("submit=%d receipt=%d", len(jobs.submitted), len(*receipts))
	}
	again := decodeReceipt(t, (*receipts)[1])
	if again.ResultStatus != resultSuccess || again.TaskID != sampleNotice().TaskID || again.ResultMessage != msgSuccess {
		t.Fatalf("%+v", again)
	}
}

func TestHandleBuildFailureSkipsUpload(t *testing.T) {
	svc, jobs, receipts, mails := setupIntake(t)
	jobs.status = job.StatusFailed
	uploads := 0
	svc.upload = func(context.Context, project.Project, string, string) error {
		uploads++
		return nil
	}
	if err := svc.Handle(context.Background(), mustNotice(sampleNotice())); err != nil {
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
	svc.upload = func(context.Context, project.Project, string, string) error {
		return errNoWar
	}
	if err := svc.Handle(context.Background(), mustNotice(sampleNotice())); err != nil {
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
	var remote string
	svc.upload = func(_ context.Context, _ project.Project, local, dest string) error {
		remote = dest
		if filepath.Base(local) != "app.war" {
			t.Fatalf("local %s", local)
		}
		return nil
	}
	if err := svc.Handle(context.Background(), mustNotice(sampleNotice())); err != nil {
		t.Fatal(err)
	}
	if remote != "/开发/S01/app.war" {
		t.Fatal(remote)
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
	return svc, jobs, &receipts, mails
}

func TestPickDataHubProject(t *testing.T) {
	one := []project.Project{{RecID: "a", ProjectName: "svc", ProjectType: "maven", IsEnabled: 1}}
	got, err := pickDataHubProject(one)
	if err != nil || got.RecID != "a" {
		t.Fatalf("%+v %v", got, err)
	}
	many := []project.Project{
		{RecID: "ui", ProjectName: "ui", ProjectType: "vue", IsEnabled: 1},
		{RecID: "dh", ProjectName: "datahub", ProjectType: "maven", IsEnabled: 1},
		{RecID: "lib", ProjectName: "lib", ProjectType: "maven", IsEnabled: 1},
	}
	got, err = pickDataHubProject(many)
	if err != nil || got.RecID != "dh" {
		t.Fatalf("prefer name datahub: %+v %v", got, err)
	}
	amb := []project.Project{
		{RecID: "a", ProjectName: "a", ProjectType: "maven", IsEnabled: 1},
		{RecID: "b", ProjectName: "b", ProjectType: "maven", IsEnabled: 1},
	}
	if _, err := pickDataHubProject(amb); err != errAmbiguousProject {
		t.Fatalf("err=%v", err)
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
	return map[string]string{
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
}

func decodeReceipt(t *testing.T, raw []byte) Receipt {
	t.Helper()
	var got Receipt
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	return got
}
