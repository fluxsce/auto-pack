package projectrelease

import "testing"

func TestOriginBranchRef(t *testing.T) {
	if got := originBranchRef("release/V9.0.0-P01"); got != "origin/release/V9.0.0-P01" {
		t.Fatalf("got %q", got)
	}
}

func TestOriginRemoteBranchRef(t *testing.T) {
	if got := originRemoteBranchRef("release/V9.0.0-P01"); got != "refs/remotes/origin/release/V9.0.0-P01" {
		t.Fatalf("got %q", got)
	}
}
