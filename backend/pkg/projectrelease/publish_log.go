package projectrelease

import (
	"fmt"

	"auto-pack/backend/pkg/redact"
)

type publishLogger struct {
	steps *[]string
}

func newPublishLogger(steps *[]string) *publishLogger {
	return &publishLogger{steps: steps}
}

func (l *publishLogger) step(format string, args ...any) {
	if l == nil || l.steps == nil {
		return
	}
	*l.steps = append(*l.steps, redact.Redact(fmt.Sprintf(format, args...)))
}
