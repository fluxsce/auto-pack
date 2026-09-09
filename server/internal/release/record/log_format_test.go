package record

import (
	"strings"
	"testing"
	"time"
)

func TestFormatBatchReleaseLogPutsOutcomeFirst(t *testing.T) {
	fixed := time.Date(2026, 9, 9, 21, 0, 0, 0, time.UTC)
	out := FormatBatchReleaseLog(BatchLogParams{
		StartLabel:  "开始自动发版",
		DoneLabel:   "自动发版结束",
		SummaryLine: "成功 2，跳过 1，失败 0，Tag V9.0.0-P08",
		Final:       true,
		Released:    2,
		Skipped:     1,
		Failed:      0,
		Timestamp:   fixed,
		Results: []Result{
			{ProjectName: "app", Status: ResultOK, Message: "released"},
		},
	})
	first := strings.SplitN(out, "\n", 2)[0]
	if first != "[成功] 成功 2，跳过 1，失败 0，Tag V9.0.0-P08" {
		t.Fatalf("first line = %q", first)
	}
	if !strings.Contains(out, "[成功] app") {
		t.Fatalf("missing project line\n%s", out)
	}
}

func TestOutcomeLabel(t *testing.T) {
	if got := OutcomeLabel(false, true, 1, 0); got != "进行中" {
		t.Fatal(got)
	}
	if got := OutcomeLabel(true, false, 2, 0); got != "成功" {
		t.Fatal(got)
	}
	if got := OutcomeLabel(true, false, 1, 1); got != "部分失败" {
		t.Fatal(got)
	}
	if got := OutcomeLabel(true, false, 0, 2); got != "失败" {
		t.Fatal(got)
	}
}
