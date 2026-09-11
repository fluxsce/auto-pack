package runner

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"auto-pack/server/internal/job"
	"auto-pack/server/internal/project"
)

func TestStepPostKeepsAutoBuildEmail(t *testing.T) {
	var buf bytes.Buffer
	e := &Engine{}
	prj := project.Project{RecipeJSON: `{"post_actions":[{"kind":"email","when":"always","to":"ops@x.com"}]}`}
	j := job.Job{PayloadJSON: `{"origin":"auto_build"}`}
	if err := e.stepPost(context.Background(), j, prj, job.Payload{}, t.TempDir(), &buf, true); err != nil {
		t.Fatal(err)
	}
	got := buf.String()
	if strings.Contains(got, "工作空间批次不按工程发信") {
		t.Fatalf("auto_build should keep post email\n%s", got)
	}
	if !strings.Contains(got, "邮件通知：1 条留到作业结束，按后置条件发送") {
		t.Fatalf("%s", got)
	}
}

func TestStepPostLogsDeferredEmail(t *testing.T) {
	var buf bytes.Buffer
	e := &Engine{}
	prj := project.Project{RecipeJSON: `{"post_actions":[{"kind":"email","when":"success","to":"ops@x.com","subject":"build ok"}]}`}
	if err := e.stepPost(context.Background(), job.Job{}, prj, job.Payload{}, t.TempDir(), &buf, true); err != nil {
		t.Fatal(err)
	}
	got := buf.String()
	if !strings.Contains(got, "邮件通知：1 条留到作业结束，按后置条件发送") {
		t.Fatalf("%s", got)
	}
}

func TestStepPostLogsShellCommand(t *testing.T) {
	var buf bytes.Buffer
	e := &Engine{}
	prj := project.Project{RecipeJSON: `{"post_actions":[{"kind":"shell","command":"echo hello-post"}]}`}
	if err := e.stepPost(context.Background(), job.Job{}, prj, job.Payload{}, t.TempDir(), &buf, true); err != nil {
		t.Fatal(err)
	}
	got := buf.String()
	if !strings.Contains(got, "执行命令 echo hello-post") || !strings.Contains(got, "脚本退出 0") {
		t.Fatalf("%s", got)
	}
}

func TestStepPostMatchesWhen(t *testing.T) {
	e := &Engine{}
	prj := project.Project{RecipeJSON: `{"post_actions":[
		{"kind":"shell","when":"success","command":"echo only-ok"},
		{"kind":"shell","when":"failure","command":"echo only-fail"},
		{"kind":"shell","when":"always","command":"echo always"}
	]}`}

	var okBuf bytes.Buffer
	if err := e.stepPost(context.Background(), job.Job{}, prj, job.Payload{}, t.TempDir(), &okBuf, true); err != nil {
		t.Fatal(err)
	}
	okLog := okBuf.String()
	if !strings.Contains(okLog, "执行命令 echo only-ok") || !strings.Contains(okLog, "执行命令 echo always") {
		t.Fatalf("success path\n%s", okLog)
	}
	if strings.Contains(okLog, "执行命令 echo only-fail") || !strings.Contains(okLog, "跳过（仅失败，本次作业成功）") {
		t.Fatalf("success should skip failure-only\n%s", okLog)
	}

	var failBuf bytes.Buffer
	if err := e.stepPost(context.Background(), job.Job{}, prj, job.Payload{}, t.TempDir(), &failBuf, false); err != nil {
		t.Fatal(err)
	}
	failLog := failBuf.String()
	if !strings.Contains(failLog, "执行命令 echo only-fail") || !strings.Contains(failLog, "执行命令 echo always") {
		t.Fatalf("failure path\n%s", failLog)
	}
	if strings.Contains(failLog, "执行命令 echo only-ok") || !strings.Contains(failLog, "跳过（仅成功，本次作业失败）") {
		t.Fatalf("failure should skip success-only\n%s", failLog)
	}
}
