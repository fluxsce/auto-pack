package record

import (
	"io"
	"os"
	"strings"
)

const maxLogTail = 512 << 10

// WriteLog replaces the persisted release log body.
func WriteLog(path, content string) error {
	if strings.TrimSpace(path) == "" {
		return nil
	}
	return os.WriteFile(path, []byte(content+"\n"), 0o644)
}

// ReadLogTail returns content from offset, the file size, and the offset actually used.
func ReadLogTail(path string, offset int64) (content string, size int64, used int64, err error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", 0, 0, nil
		}
		return "", 0, 0, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return "", 0, 0, err
	}
	size = info.Size()
	used = offset
	if used < 0 || used > size {
		used = 0
	}
	if used == size {
		return "", size, used, nil
	}
	if used > 0 {
		if _, err := f.Seek(used, io.SeekStart); err != nil {
			return "", 0, 0, err
		}
	}
	b, err := io.ReadAll(f)
	if err != nil {
		return "", 0, 0, err
	}
	if len(b) > maxLogTail {
		b = b[len(b)-maxLogTail:]
	}
	return string(b), size, used, nil
}
