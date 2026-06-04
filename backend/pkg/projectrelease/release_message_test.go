package projectrelease

import "testing"

func TestFormatReleaseMessage_withTarget(t *testing.T) {
	msg := formatReleaseMessage("develop", "V9.0.0-P08", true, "release/V9.0.0-P08")
	want := "released to develop, target branch release/V9.0.0-P08, git tag V9.0.0-P08"
	if msg != want {
		t.Fatalf("got %q want %q", msg, want)
	}
}
