package runner

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadLogOffset(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	e := &Engine{dataDir: dir}
	jobID := "job-log-off"
	if err := os.MkdirAll(filepath.Join(dir, "jobs", jobID), 0o755); err != nil {
		t.Fatal(err)
	}
	body := []byte("hello\nworld\n")
	if err := os.WriteFile(filepath.Join(dir, "jobs", jobID, "job.log"), body, 0o644); err != nil {
		t.Fatal(err)
	}
	text, size, used, err := e.ReadLog(jobID, 0)
	if err != nil || text != string(body) || size != int64(len(body)) || used != 0 {
		t.Fatalf("full text=%q size=%d used=%d err=%v", text, size, used, err)
	}
	text, size, used, err = e.ReadLog(jobID, 6)
	if err != nil || text != "world\n" || size != int64(len(body)) || used != 6 {
		t.Fatalf("tail text=%q size=%d used=%d err=%v", text, size, used, err)
	}
	text, size, used, err = e.ReadLog(jobID, int64(len(body)))
	if err != nil || text != "" || used != int64(len(body)) {
		t.Fatalf("eof text=%q size=%d used=%d err=%v", text, size, used, err)
	}
	text, size, used, err = e.ReadLog(jobID, 99)
	if err != nil || text != string(body) || used != 0 {
		t.Fatalf("reset text=%q size=%d used=%d err=%v", text, size, used, err)
	}
}

func TestLogLineHasTimestamp(t *testing.T) {
	var buf bytes.Buffer
	logLine(&buf, "%d. %s：开始", 1, postActionTitle("email"))
	got := buf.String()
	if !strings.Contains(got, "1. 邮件通知：开始") || !strings.HasPrefix(got, "[") {
		t.Fatal(got)
	}
}
