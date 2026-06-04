package shell

import (
	"runtime"
	"testing"
	"unicode/utf8"

	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
)

func TestDecodeProcessOutputUTF8Passthrough(t *testing.T) {
	in := "hello 世界"
	got := DecodeProcessOutput([]byte(in))
	if got != in {
		t.Fatalf("got %q want %q", got, in)
	}
}

func TestDecodeProcessOutputGBKOnWindows(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("GBK fallback applies on Windows only")
	}
	msg := "使用了已过时的 API"
	gbk, _, err := transform.Bytes(simplifiedchinese.GBK.NewEncoder(), []byte(msg))
	if err != nil {
		t.Fatal(err)
	}
	if utf8.Valid(gbk) {
		t.Fatal("expected GBK sample to be non-UTF-8")
	}

	prefix := []byte("[INFO] ")
	mixed := append(append([]byte{}, prefix...), gbk...)

	got := DecodeProcessOutput(mixed)
	if !utf8.ValidString(got) {
		t.Fatalf("expected valid UTF-8, got %q", got)
	}
	if got != string(prefix)+msg {
		t.Fatalf("got %q", got)
	}
}
