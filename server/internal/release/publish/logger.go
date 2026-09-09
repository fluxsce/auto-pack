// Package publish 在发版工作区执行 Git checkout/commit/tag/push，并记录步骤日志。
package publish

import "fmt"

// Logger appends human-readable steps onto a result slice.
type Logger struct {
	steps *[]string
}

// NewLogger binds a step list for Git publish progress.
func NewLogger(steps *[]string) *Logger {
	if steps == nil {
		empty := []string{}
		steps = &empty
	}
	return &Logger{steps: steps}
}

// Step appends one formatted line to the step list.
func (l *Logger) Step(format string, args ...any) {
	if l == nil || l.steps == nil {
		return
	}
	*l.steps = append(*l.steps, fmt.Sprintf(format, args...))
}
