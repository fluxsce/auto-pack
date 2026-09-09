package utils

import (
	"io"
	"os"
	"unicode/utf8"
)

const defaultReadLimit = 256 << 10

// ReadText 读取 root 下 rel 的文本。超过 maxBytes 时截断。
func ReadText(root, rel string, maxBytes int) (content string, truncated bool, err error) {
	dest, cleaned, err := Resolve(root, rel)
	if err != nil {
		return "", false, err
	}
	if cleaned == "" {
		return "", false, ErrNotFile
	}
	st, err := os.Stat(dest)
	if err != nil {
		return "", false, err
	}
	if st.IsDir() {
		return "", false, ErrNotFile
	}
	if maxBytes <= 0 {
		maxBytes = defaultReadLimit
	}
	f, err := os.Open(dest)
	if err != nil {
		return "", false, err
	}
	defer f.Close()
	raw, err := io.ReadAll(io.LimitReader(f, int64(maxBytes)+1))
	if err != nil {
		return "", false, err
	}
	truncated = len(raw) > maxBytes
	if truncated {
		raw = raw[:maxBytes]
	}
	if !utf8.Valid(raw) {
		return "", false, ErrNotText
	}
	return string(raw), truncated, nil
}
