package packrun

import (
	"fmt"
	"os"
	"strings"
)

const (
	maxInlineBuildLogBytes = 256 * 1024
	maxAttachBuildLogBytes = 4 * 1024 * 1024
)

// loadBuildLogForNotify reads the on-disk build log for failure notification emails.
// Small logs are returned inline; larger logs are returned as attach bytes only.
func loadBuildLogForNotify(logPath string) (inline string, attach []byte, truncated bool) {
	logPath = strings.TrimSpace(logPath)
	if logPath == "" {
		return "", nil, false
	}
	b, err := os.ReadFile(logPath)
	if err != nil {
		return fmt.Sprintf("(read build log failed: %v)", err), nil, false
	}
	if len(b) == 0 {
		return "", nil, false
	}
	truncated = len(b) > maxAttachBuildLogBytes
	if truncated {
		b = b[:maxAttachBuildLogBytes]
	}
	if len(b) <= maxInlineBuildLogBytes {
		return string(b), nil, truncated
	}
	out := make([]byte, len(b))
	copy(out, b)
	return "", out, truncated
}
