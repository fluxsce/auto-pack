package runner

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"auto-pack/server/internal/job"
	"auto-pack/server/internal/project"
)

func TestStepPostLogsDeferredEmail(t *testing.T) {
	var buf bytes.Buffer
	e := &Engine{}
	prj := project.Project{RecipeJSON: `{"post_actions":[{"kind":"email","when":"success","to":"ops@x.com","subject":"build ok"}]}`}
	if err := e.stepPost(context.Background(), job.Job{}, prj, job.Payload{}, t.TempDir(), &buf); err != nil {
		t.Fatal(err)
	}
	got := buf.String()
	for _, want := range []string{
		"邮件通知：作业结束后发送（仅成功）",
		"收件人 ops@x.com",
		"主题 build ok",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in\n%s", want, got)
		}
	}
}

func TestStepPostLogsShellCommand(t *testing.T) {
	var buf bytes.Buffer
	e := &Engine{}
	prj := project.Project{RecipeJSON: `{"post_actions":[{"kind":"shell","command":"echo hello-post"}]}`}
	if err := e.stepPost(context.Background(), job.Job{}, prj, job.Payload{}, t.TempDir(), &buf); err != nil {
		t.Fatal(err)
	}
	got := buf.String()
	if !strings.Contains(got, "执行命令 echo hello-post") || !strings.Contains(got, "脚本退出 0") {
		t.Fatalf("%s", got)
	}
}
