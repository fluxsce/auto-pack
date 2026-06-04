package projectcfg

import (
	"fmt"
	"strings"

	"github.com/robfig/cron/v3"
)

// BuildSchedule is cron-based automatic build configuration.
type BuildSchedule struct {
	Enabled bool   `json:"enabled"`
	Cron    string `json:"cron"`
}

// BuildTriggers controls pipeline dependencies and scheduled builds.
type BuildTriggers struct {
	DependsOn []string      `json:"depends_on"`
	Schedule  BuildSchedule `json:"schedule"`
}

// ParseBuildTriggers extracts build_triggers from raw config JSON map fields.
func ParseBuildTriggers(raw map[string]any) BuildTriggers {
	out := BuildTriggers{DependsOn: []string{}}
	if raw == nil {
		return out
	}
	if arr, ok := raw["depends_on"].([]any); ok {
		for _, v := range arr {
			if s := strings.TrimSpace(fmt.Sprint(v)); s != "" {
				out.DependsOn = append(out.DependsOn, s)
			}
		}
	}
	if sched, ok := raw["schedule"].(map[string]any); ok {
		if sched["enabled"] == true {
			out.Schedule.Enabled = true
		}
		out.Schedule.Cron = strings.TrimSpace(fmt.Sprint(sched["cron"]))
	}
	return out
}

// ValidateBuildTriggers checks dependency and cron expressions.
func ValidateBuildTriggers(selfRecID string, t BuildTriggers) error {
	seen := map[string]bool{}
	for _, id := range t.DependsOn {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if id == selfRecID {
			return fmt.Errorf("build dependency cannot include this project itself")
		}
		if seen[id] {
			return fmt.Errorf("duplicate build dependency %s", id)
		}
		seen[id] = true
	}
	if t.Schedule.Enabled {
		cronExpr := strings.TrimSpace(t.Schedule.Cron)
		if cronExpr == "" {
			return fmt.Errorf("schedule cron expression is required when schedule is enabled")
		}
		if _, err := cron.ParseStandard(cronExpr); err != nil {
			return fmt.Errorf("invalid schedule cron: %w", err)
		}
	}
	return nil
}

// HasDependency reports whether recID is listed as an upstream module.
func (t BuildTriggers) HasDependency(recID string) bool {
	recID = strings.TrimSpace(recID)
	for _, id := range t.DependsOn {
		if strings.TrimSpace(id) == recID {
			return true
		}
	}
	return false
}
