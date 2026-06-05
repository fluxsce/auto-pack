package projectrelease

import (
	"slices"
	"testing"
)

func TestGitTagNameFromReleaseBranch(t *testing.T) {
	if got := gitTagNameFromReleaseBranch("release/V9.0.0-P01"); got != "V9.0.0-P01" {
		t.Fatalf("got %q", got)
	}
}

func TestGitTagNamesForCheckout_onlyBranchDerivedTag(t *testing.T) {
	names := gitTagNamesForCheckout("release/V9.0.0-P01", "V9.0.0-P01-01")
	if len(names) != 1 || names[0] != "V9.0.0-P01" {
		t.Fatalf("got %v", names)
	}
}

func TestGitTagRefCandidates_tagOnly(t *testing.T) {
	got := gitTagRefCandidates("V9.0.0-P01")
	want := []string{"refs/tags/V9.0.0-P01", "tags/V9.0.0-P01", "V9.0.0-P01"}
	if len(got) != len(want) {
		t.Fatalf("got %v want %v", got, want)
	}
	for _, w := range want {
		if !slices.Contains(got, w) {
			t.Fatalf("missing %q", w)
		}
	}
	if slices.Contains(got, "origin/V9.0.0-P01") {
		t.Fatal("should not include branch ref origin/")
	}
}

func TestValidateVersionTag(t *testing.T) {
	if err := ValidateVersionTag("V9.0.0-P08"); err != nil {
		t.Fatalf("valid: %v", err)
	}
	if err := ValidateVersionTag("bad tag"); err == nil {
		t.Fatal("expected error for space")
	}
}

func TestResolveGitTagRefsForCheckout(t *testing.T) {
	refs := resolveGitTagRefsForCheckout("release/V9.0.0-P01", "V9.0.0-P01-01")
	if len(refs) != 3 {
		t.Fatalf("expected 3 tag refs, got %v", refs)
	}
	if refs[0] != "refs/tags/V9.0.0-P01" {
		t.Fatalf("first ref should be refs/tags/V9.0.0-P01, got %v", refs)
	}
}
