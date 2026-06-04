package projectrelease

import "testing"

func TestDefaultReleaseNote(t *testing.T) {
	if got := DefaultReleaseNote("V9.0.0-P01-01"); got != "V9.0.0-P01-01自动发版说明" {
		t.Fatalf("got %q", got)
	}
}

func TestResolveCommitMessage(t *testing.T) {
	if got := ResolveCommitMessage("", "V9.0.0-P01-01"); got != "V9.0.0-P01-01自动发版说明" {
		t.Fatalf("got %q", got)
	}
	if got := ResolveCommitMessage("custom", "V9.0.0-P01-01"); got != "custom" {
		t.Fatalf("got %q", got)
	}
}
