package packrun

import "strings"

// TriggerMode controls whether upstream build dependencies are enforced.
type TriggerMode string

const (
	// TriggerManual skips dependency checks (UI “build now”).
	TriggerManual TriggerMode = "manual"
	// TriggerPipeline is used when an upstream module finished or the dependency chain fires.
	TriggerPipeline TriggerMode = "pipeline"
	// TriggerSchedule is used by cron; dependencies are enforced like pipeline.
	TriggerSchedule TriggerMode = "schedule"
)

func (m TriggerMode) String() string {
	if m == "" {
		return string(TriggerManual)
	}
	return string(m)
}

// ParseTriggerModeQuery parses ?trigger= manual|pipeline|schedule.
func ParseTriggerModeQuery(raw string) TriggerMode {
	return parseTriggerMode(raw)
}

func parseTriggerMode(raw string) TriggerMode {
	switch TriggerMode(strings.TrimSpace(raw)) {
	case TriggerPipeline:
		return TriggerPipeline
	case TriggerSchedule:
		return TriggerSchedule
	default:
		return TriggerManual
	}
}
