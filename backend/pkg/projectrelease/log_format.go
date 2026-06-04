package projectrelease

import (
	"fmt"
	"strings"
	"time"
)

// BatchLogParams describes the header of a workspace batch release log.
type BatchLogParams struct {
	ReleaseBranch     string
	Tag               string
	TargetBranch      string
	ReleaseNote       string
	ReleaseDate       string
	UpdatePom         bool
	UpdatePomOnLabel  string
	UpdatePomOffLabel string
	StartLabel        string
	DoneLabel         string
	ProgressLabel     string
	ParamsLine        string
	NoteLine          string
	DateLine          string
	PomLine           string
	SummaryLine       string
	NoStepsLabel      string
	NoMessageLabel    string
	InProgressLabel   string
	InProgressProject string
	Final             bool
	Running           bool
	Released          int
	Skipped           int
	Failed            int
	Results           []ReleaseResult
	Timestamp         time.Time
}

// StatusLabel returns a short bracket label for log highlighting.
func StatusLabel(status string) string {
	switch status {
	case "ok":
		return "成功"
	case "skipped":
		return "跳过"
	default:
		return "失败"
	}
}

// FormatBatchReleaseLog renders the full persisted / polled release log body.
func FormatBatchReleaseLog(p BatchLogParams) string {
	ts := p.Timestamp
	if ts.IsZero() {
		ts = time.Now()
	}
	stamp := ts.Format("2006-01-02 15:04:05")
	var lines []string

	if p.StartLabel != "" {
		lines = append(lines, fmt.Sprintf("[%s] %s", stamp, p.StartLabel))
	}
	if pl := strings.TrimSpace(p.ParamsLine); pl != "" {
		lines = append(lines, pl)
	}
	if nl := strings.TrimSpace(p.NoteLine); nl != "" {
		lines = append(lines, nl)
	}
	if dl := strings.TrimSpace(p.DateLine); dl != "" {
		lines = append(lines, dl)
	}
	if pom := strings.TrimSpace(p.PomLine); pom != "" {
		lines = append(lines, pom)
	}

	body := formatReleaseProgressBody(p, stamp)
	if body != "" {
		if len(lines) > 0 {
			lines = append(lines, "")
		}
		lines = append(lines, body)
	}
	return strings.TrimRight(strings.Join(lines, "\n"), "\n")
}

func formatReleaseProgressBody(p BatchLogParams, stamp string) string {
	var body []string
	if p.Final {
		body = append(body, fmt.Sprintf("[%s] %s", stamp, p.DoneLabel))
		if sl := strings.TrimSpace(p.SummaryLine); sl != "" {
			body = append(body, sl)
		}
		body = append(body, "")
	} else if p.Running && p.ProgressLabel != "" {
		body = append(body, fmt.Sprintf("[%s] %s", stamp, p.ProgressLabel))
		body = append(body, "")
	}
	for _, item := range p.Results {
		body = append(body, formatProjectResultLines(item, p.NoStepsLabel, p.NoMessageLabel)...)
		body = append(body, "")
	}
	if p.Running {
		proj := strings.TrimSpace(p.InProgressProject)
		if proj != "" && p.InProgressLabel != "" {
			body = append(body, fmt.Sprintf("[…] %s", fmt.Sprintf(p.InProgressLabel, proj)))
		}
	}
	return strings.TrimRight(strings.Join(body, "\n"), "\n")
}

func formatProjectResultLines(item ReleaseResult, noSteps, noMessage string) []string {
	lines := []string{fmt.Sprintf("[%s] %s", StatusLabel(item.Status), item.ProjectName)}
	steps := item.Steps
	if len(steps) > 0 {
		for _, step := range steps {
			lines = append(lines, "  · "+step)
		}
	} else if item.Status != "skipped" && strings.TrimSpace(noSteps) != "" {
		lines = append(lines, "  · "+noSteps)
	}
	msg := strings.TrimSpace(item.Message)
	if msg != "" {
		lines = append(lines, "  → "+msg)
	} else if item.Status == "failed" && strings.TrimSpace(noMessage) != "" {
		lines = append(lines, "  → "+noMessage)
	}
	return lines
}
