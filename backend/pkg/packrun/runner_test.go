package packrun

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadLogTail_EOFAtOffset(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.log")
	if err := os.WriteFile(path, []byte("hello world"), 0o644); err != nil {
		t.Fatal(err)
	}

	content, total, err := ReadLogTail(path, 11, 1024)
	if err != nil {
		t.Fatalf("expected no error at EOF, got %v", err)
	}
	if content != "" {
		t.Fatalf("expected empty content at EOF, got %q", content)
	}
	if total != 11 {
		t.Fatalf("expected total 11, got %d", total)
	}
}

func TestReadLogTail_PartialRead(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.log")
	data := []byte("0123456789")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}

	content, total, err := ReadLogTail(path, 5, 1024)
	if err != nil {
		t.Fatal(err)
	}
	if content != "56789" {
		t.Fatalf("got %q", content)
	}
	if total != int64(len(data)) {
		t.Fatalf("total %d", total)
	}
}
