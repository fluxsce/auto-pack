package projectrelease

import "fmt"

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
	*l.steps = append(*l.steps, fmt.Sprintf(format, args...))
}
