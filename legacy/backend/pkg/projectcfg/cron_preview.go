package projectcfg

import (
	"fmt"
	"strings"
	"time"

	"github.com/robfig/cron/v3"
)

const (
	DefaultCronPreviewCount = 3
	MaxCronPreviewCount     = 10
)

// PreviewCronSchedule returns the next N run times for a standard 5-field cron expression.
func PreviewCronSchedule(cronExpr string, count int) ([]time.Time, error) {
	expr := strings.TrimSpace(cronExpr)
	if expr == "" {
		return nil, fmt.Errorf("cron expression is required")
	}
	if count <= 0 {
		count = DefaultCronPreviewCount
	}
	if count > MaxCronPreviewCount {
		count = MaxCronPreviewCount
	}
	sched, err := cron.ParseStandard(expr)
	if err != nil {
		return nil, fmt.Errorf("invalid schedule cron: %w", err)
	}
	now := time.Now()
	runs := make([]time.Time, 0, count)
	t := now
	for i := 0; i < count; i++ {
		t = sched.Next(t)
		runs = append(runs, t)
	}
	return runs, nil
}
