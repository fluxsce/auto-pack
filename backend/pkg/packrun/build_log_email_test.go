package packrun

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadBuildLogForNotify_inline(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "build.log")
	if err := os.WriteFile(path, []byte("line1\nline2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	inline, attach, truncated := loadBuildLogForNotify(path)
	if truncated || len(attach) > 0 {
		t.Fatalf("want inline only, got truncated=%v attach=%d", truncated, len(attach))
	}
	if !strings.Contains(inline, "line2") {
		t.Fatalf("inline=%q", inline)
	}
}

func TestLoadBuildLogForNotify_attachment(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "build.log")
	big := make([]byte, maxInlineBuildLogBytes+1024)
	for i := range big {
		big[i] = 'x'
	}
	if err := os.WriteFile(path, big, 0o644); err != nil {
		t.Fatal(err)
	}
	inline, attach, truncated := loadBuildLogForNotify(path)
	if inline != "" || len(attach) != len(big) || truncated {
		t.Fatalf("inline len=%d attach=%d truncated=%v", len(inline), len(attach), truncated)
	}
}
