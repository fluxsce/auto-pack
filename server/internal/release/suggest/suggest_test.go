package suggest

import "testing"

func TestSuggestNextVersionTag_firstRevision(t *testing.T) {
	code := "V9.0.0-P08"
	got := SuggestNextVersionTag(code, []string{"V9.0.0-P08"})
	if got != "V9.0.0-P08-01" {
		t.Fatalf("got %q", got)
	}
}

func TestSuggestNextVersionTag_revision(t *testing.T) {
	code := "V9.0.0-P08"
	tags := []string{"V9.0.0-P08", "V9.0.0-P08-01", "V9.0.0-P08-02"}
	got := SuggestNextVersionTag(code, tags)
	if got != "V9.0.0-P08-03" {
		t.Fatalf("got %q", got)
	}
}

func TestSuggestNextVersionTag_patch(t *testing.T) {
	code := "V9.0.0"
	tags := []string{"V9.0.0-P01", "V9.0.0-P08", "V9.0.0-P02"}
	got := SuggestNextVersionTag(code, tags)
	if got != "V9.0.0-P09" {
		t.Fatalf("got %q", got)
	}
}

func TestSuggestNextVersionTag_empty(t *testing.T) {
	if got := SuggestNextVersionTag("V9.0.0-P08", nil); got != "V9.0.0-P08" {
		t.Fatalf("got %q", got)
	}
}

func TestSuggestNextVersionTag(t *testing.T) {
	if got := SuggestNextVersionTag("V9.0.0-P08", []string{"V9.0.0-P08"}); got != "V9.0.0-P08-01" {
		t.Fatalf("got %q", got)
	}
	if got := SuggestNextVersionTag("V9.0.0-P08", []string{"V9.0.0-P08-02"}); got != "V9.0.0-P08-03" {
		t.Fatalf("got %q", got)
	}
	if got := SuggestNextVersionTag("V9.0.0", []string{"V9.0.0-P08"}); got != "V9.0.0-P09" {
		t.Fatalf("got %q", got)
	}
}

func TestDefaultReleaseBranch(t *testing.T) {
	if got := DefaultReleaseBranch("V9.0.0-P08"); got != "release/V9.0.0-P08" {
		t.Fatalf("got %q", got)
	}
}
