package runner

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"auto-pack/server/internal/job"
	"auto-pack/server/internal/migrate"
	"auto-pack/server/internal/project"
	"auto-pack/server/internal/store"
)

func TestNotifyKeepsAutoBuildMail(t *testing.T) {
	var buf bytes.Buffer
	e := &Engine{}
	e.notify(context.Background(), job.Job{JobType: job.TypeBuild, PayloadJSON: `{"origin":"auto_build"}`}, false, "exit status 1", &buf)
	got := buf.String()
	if strings.Contains(got, "工作空间批次不按工程发信") {
		t.Fatalf("auto_build should still notify\n%s", got)
	}
	if !strings.Contains(got, "== 邮件通知 ==") || !strings.Contains(got, "邮件通知：跳过（空间未配置 SMTP）") {
		t.Fatalf("%s", got)
	}
}

func TestNotifySkipsWorkspaceFallbackWhenWhenMisses(t *testing.T) {
	var buf bytes.Buffer
	e := &Engine{}
	prj := project.Project{RecipeJSON: `{"post_actions":[{"kind":"email","when":"failure","to":"ops@x.com"}]}`}
	e.notifyWithProject(context.Background(), job.Job{JobType: job.TypeBuild}, prj, true, "", &buf)
	got := buf.String()
	if !strings.Contains(got, "邮件通知：跳过（仅失败，本次作业成功）") {
		t.Fatalf("%s", got)
	}
	if strings.Contains(got, "尝试空间默认 SMTP") || strings.Contains(got, "空间未配置 SMTP") {
		t.Fatalf("when-miss should not fall back\n%s", got)
	}
}

func TestNotifyLogsWorkspaceSMTPMissing(t *testing.T) {
	var buf bytes.Buffer
	e := &Engine{}
	e.notify(context.Background(), job.Job{JobType: job.TypeBuild}, true, "", &buf)
	got := buf.String()
	if !strings.Contains(got, "== 邮件通知 ==") || !strings.Contains(got, "邮件通知：跳过（空间未配置 SMTP）") {
		t.Fatalf("%s", got)
	}
}

func TestSendActionMailRequiresSMTPEnv(t *testing.T) {
	err := (&Engine{}).sendActionMail(context.Background(), job.Job{}, project.Project{}, project.PostAction{Kind: "email", To: "a@b.c"}, true, "", io.Discard)
	if err == nil || !strings.Contains(err.Error(), "smtp env not found") {
		t.Fatalf("%v", err)
	}
}

func TestNotifyMailToPrefersPayload(t *testing.T) {
	got := notifyMailTo(job.Payload{MailTo: "a@b.c,c@d.e"}, "ops@x.com", "smtp@x.com")
	if got != "a@b.c,c@d.e" {
		t.Fatal(got)
	}
	if notifyMailTo(job.Payload{}, "ops@x.com", "smtp@x.com") != "ops@x.com" {
		t.Fatal("action")
	}
	if notifyMailTo(job.Payload{}, "", "smtp@x.com") != "smtp@x.com" {
		t.Fatal("smtp")
	}
}

func TestNotifyHoldsPackMailUntilUpload(t *testing.T) {
	var buf bytes.Buffer
	e := &Engine{}
	e.notify(context.Background(), job.Job{JobType: job.TypeBuild, PayloadJSON: `{"hold_mail":true,"task_ref":"T1"}`}, true, "", &buf)
	if !strings.Contains(buf.String(), "待安装包上传后发送") || strings.Contains(buf.String(), "未配置 SMTP") {
		t.Fatalf("%s", buf.String())
	}
}

func TestComposePackMailIncludesDownloadLink(t *testing.T) {
	prj := project.Project{ProjectName: "datahub", RepoBranch: "main"}
	payload := job.Payload{
		TaskRef:       "TSK-1",
		TaskNo:        "1",
		CustomerCode:  "C01",
		Version:       "1.2.3",
		Branch:        "release/customer",
		RemotePath:    "/开发/S01/app.war",
		MailNote:      "自动打包完成，war 已推送到项目路径",
		DownloadURL:   "https://pack.example/autopack/api/pack/abc",
		DownloadUntil: "2026-10-15 14:00:00",
	}
	subject, body := composePackMail(prj, payload, true, "")
	if !strings.Contains(subject, "datahub") || !strings.Contains(subject, "打包已完成") {
		t.Fatal(subject)
	}
	for _, want := range []string{"任务号：1", "客户代码：C01", "版本：1.2.3", "项目路径：/开发/S01/app.war", "下载链接：https://pack.example/autopack/api/pack/abc", "有效期至：2026-10-15 14:00:00"} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing %s\n%s", want, body)
		}
	}
	if strings.Contains(body, "仓库：") || strings.Contains(body, "作业编号：") {
		t.Fatalf("%s", body)
	}
}

func TestComposeNotifyMailIncludesKeyFields(t *testing.T) {
	j := job.Job{
		RecID:          "job-1",
		WorkspaceRecID: "ws-1",
		JobType:        job.TypeBuild,
		PayloadJSON:    `{"branch":"master"}`,
		StartedAt:      "2026-09-08 16:59:13",
		FinishedAt:     "2026-09-08 17:00:34",
	}
	prj := project.Project{
		ProjectName: "testServer",
		ProjectType: "maven",
		RepoURL:     "http://gitlab.example/testServer.git",
		RepoBranch:  "develop",
	}
	subject, body := composeNotifyMail(j, prj, "研发空间", project.PostAction{}, true, "")
	if !strings.Contains(subject, "testServer") || !strings.Contains(subject, "master") || !strings.Contains(subject, "已成功") {
		t.Fatalf("subject=%q", subject)
	}
	for _, want := range []string{
		"工程名称：testServer",
		"分支：master",
		"构建状态：已成功 (succeeded)",
		"工作空间：研发空间",
		"仓库：http://gitlab.example/testServer.git",
		"作业类型：构建",
		"作业编号：job-1",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing %q in\n%s", want, body)
		}
	}
}

func TestComposeNotifyMailFailedUsesProjectBranch(t *testing.T) {
	j := job.Job{RecID: "j2", JobType: job.TypeBuild}
	prj := project.Project{ProjectName: "app", RepoBranch: "release"}
	subject, body := composeNotifyMail(j, prj, "", project.PostAction{}, false, "mvn failed")
	if !strings.Contains(subject, "失败") || !strings.Contains(subject, "release") {
		t.Fatalf("subject=%q", subject)
	}
	if !strings.Contains(body, "构建状态：失败 (failed)") || !strings.Contains(body, "错误信息：mvn failed") {
		t.Fatalf("body=%s", body)
	}
}

func TestAppendMailLog(t *testing.T) {
	if got := appendMailLog("status\n", "   "); got != "status\n" {
		t.Fatalf("empty tail: %q", got)
	}
	got := appendMailLog("status\n", "  ERROR compile\nBUILD FAILED  ")
	if !strings.Contains(got, "--- 构建日志 ---") || !strings.Contains(got, "BUILD FAILED") || !strings.HasSuffix(got, "\n") {
		t.Fatalf("body=%q", got)
	}
}

func TestPruneOlderThanRemovesLogAndRow(t *testing.T) {
	ctx := context.Background()
	st, err := store.Open(ctx, store.Config{Driver: "sqlite", Path: filepath.Join(t.TempDir(), "t.db")})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if _, err := migrate.Apply(ctx, st, "sqlite"); err != nil {
		t.Fatal(err)
	}
	dataDir := t.TempDir()
	old := time.Now().UTC().AddDate(0, 0, -10).Format(job.TimeLayout)
	keep := time.Now().UTC().AddDate(0, 0, -1).Format(job.TimeLayout)
	insertFinished(t, ctx, st, "old-job", "p1", job.StatusSucceeded, old)
	insertFinished(t, ctx, st, "new-job", "p1", job.StatusFailed, keep)
	oldDir := filepath.Join(dataDir, "jobs", "old-job")
	if err := os.MkdirAll(oldDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(oldDir, "job.log"), []byte("old\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	e := NewEngine(st, nil, nil, dataDir, "", 0)
	if n := e.pruneOlderThan(ctx, "p1", "ws", 7); n != 1 {
		t.Fatalf("removed=%d", n)
	}
	if _, err := job.JobByID(ctx, st, "old-job"); err == nil {
		t.Fatal("old job row should be deleted")
	}
	if _, err := os.Stat(oldDir); !os.IsNotExist(err) {
		t.Fatalf("old log dir: %v", err)
	}
	if _, err := job.JobByID(ctx, st, "new-job"); err != nil {
		t.Fatal(err)
	}
}

func TestPruneOlderThanZeroDaysKeepsRows(t *testing.T) {
	ctx := context.Background()
	st, err := store.Open(ctx, store.Config{Driver: "sqlite", Path: filepath.Join(t.TempDir(), "t.db")})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if _, err := migrate.Apply(ctx, st, "sqlite"); err != nil {
		t.Fatal(err)
	}
	old := time.Now().UTC().AddDate(0, 0, -30).Format(job.TimeLayout)
	insertFinished(t, ctx, st, "keep", "p1", job.StatusSucceeded, old)
	e := NewEngine(st, nil, nil, t.TempDir(), "", 0)
	if n := e.pruneOlderThan(ctx, "p1", "ws", 0); n != 0 {
		t.Fatalf("removed=%d", n)
	}
	if _, err := job.JobByID(ctx, st, "keep"); err != nil {
		t.Fatal(err)
	}
}

func TestPruneAllUsesProjectRetention(t *testing.T) {
	ctx := context.Background()
	st, err := store.Open(ctx, store.Config{Driver: "sqlite", Path: filepath.Join(t.TempDir(), "t.db")})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if _, err := migrate.Apply(ctx, st, "sqlite"); err != nil {
		t.Fatal(err)
	}
	q := st.Rebind(`INSERT INTO AP_PROJECT (rec_id, workspace_rec_id, project_name, project_type, recipe_json)
		VALUES (?, ?, ?, ?, ?)`)
	if _, err := st.SQL().ExecContext(ctx, q, "p1", "ws", "app", "maven", `{"log_retention_days":7}`); err != nil {
		t.Fatal(err)
	}
	old := time.Now().UTC().AddDate(0, 0, -10).Format(job.TimeLayout)
	insertFinished(t, ctx, st, "old-job", "p1", job.StatusSucceeded, old)
	e := NewEngine(st, nil, nil, t.TempDir(), "", 0)
	e.pruneAll(ctx)
	if _, err := job.JobByID(ctx, st, "old-job"); err == nil {
		t.Fatal("old job should be pruned by background sweep")
	}
}

func insertFinished(t *testing.T, ctx context.Context, st *store.Store, id, projectID, status, finished string) {
	t.Helper()
	q := st.Rebind(`INSERT INTO AP_JOB (rec_id, workspace_rec_id, project_rec_id, job_type, job_status, payload_json, created_by, finished_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if _, err := st.SQL().ExecContext(ctx, q, id, "ws", projectID, job.TypeBuild, status, "{}", "u1", finished, finished); err != nil {
		t.Fatal(err)
	}
}
