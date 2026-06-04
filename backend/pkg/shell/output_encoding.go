package shell

import (
	"bytes"
	"io"
	"runtime"
	"unicode/utf8"

	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
)

// DecodeProcessOutput normalizes subprocess bytes for UTF-8 log storage and display.
func DecodeProcessOutput(b []byte) string {
	if len(b) == 0 {
		return ""
	}
	if utf8.Valid(b) {
		return string(b)
	}
	if runtime.GOOS == "windows" {
		if decoded, err := transformBytes(simplifiedchinese.GBK.NewDecoder(), b); err == nil {
			if utf8.Valid(decoded) {
				return string(decoded)
			}
		}
	}
	return string(b)
}

func transformBytes(t transform.Transformer, b []byte) ([]byte, error) {
	return io.ReadAll(transform.NewReader(bytes.NewReader(b), t))
}
