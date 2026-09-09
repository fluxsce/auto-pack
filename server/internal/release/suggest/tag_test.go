package suggest

import "testing"

func TestRepoBranchAfterRelease(t *testing.T) {
	if got := RepoBranchAfterRelease("release/A", "develop", true); got != "develop" {
		t.Fatal(got)
	}
	if got := RepoBranchAfterRelease("release/A", "develop", false); got != "release/A" {
		t.Fatal(got)
	}
}
