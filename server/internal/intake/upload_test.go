package intake

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSelectPushFiles(t *testing.T) {
	work := t.TempDir()
	if err := os.MkdirAll(filepath.Join(work, "target"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"target/app.war", "target/other.war", "target/config.zip"} {
		if err := os.WriteFile(filepath.Join(work, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	all, err := selectPushFiles(work, nil)
	if err != nil || len(all) != 2 {
		t.Fatalf("wars=%v %v", all, err)
	}
	picked, err := selectPushFiles(work, []string{"config.zip", "app.war"})
	if err != nil || len(picked) != 2 {
		t.Fatalf("picked=%v %v", picked, err)
	}
	if filepath.Base(picked[0]) != "config.zip" || filepath.Base(picked[1]) != "app.war" {
		t.Fatalf("%v", picked)
	}
	if _, err := selectPushFiles(work, []string{"missing.jar"}); err == nil {
		t.Fatal("missing file must fail")
	}
}

func TestOSSBackupKeyUsesBAKStamp(t *testing.T) {
	at := time.Date(2026, 10, 17, 0, 49, 0, 123000000, time.UTC)
	got := ossBackupKey("开发/S01/app.war", at)
	if got != "开发/S01/BAK/20261017004900123/app.war" {
		t.Fatal(got)
	}
}
