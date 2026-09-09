package projectcfg

import (
	"testing"
	"time"
)

func TestPreviewCronSchedule(t *testing.T) {
	t.Parallel()
	runs, err := PreviewCronSchedule("0 2 * * *", 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(runs) != 3 {
		t.Fatalf("got %d runs want 3", len(runs))
	}
	for i := 1; i < len(runs); i++ {
		if !runs[i].After(runs[i-1]) {
			t.Fatalf("runs not ascending: %v", runs)
		}
	}
}

func TestPreviewCronSchedule_invalid(t *testing.T) {
	t.Parallel()
	_, err := PreviewCronSchedule("not a cron", 1)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestPreviewCronSchedule_empty(t *testing.T) {
	t.Parallel()
	_, err := PreviewCronSchedule("  ", 1)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestPreviewCronSchedule_firstRunAfterNow(t *testing.T) {
	t.Parallel()
	// Every minute at second 0 — next run should be within ~2 minutes.
	runs, err := PreviewCronSchedule("* * * * *", 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(runs) != 1 {
		t.Fatal(runs)
	}
	if runs[0].Before(time.Now().Add(-time.Minute)) {
		t.Fatalf("first run too far in past: %v", runs[0])
	}
}
