package git

import "testing"

func TestNormalizeTagName(t *testing.T) {
	if got := NormalizeTagName(" refs/tags/V9.0.0-P01 "); got != "V9.0.0-P01" {
		t.Fatalf("got %q", got)
	}
}

func TestFormatTagRef(t *testing.T) {
	if got := FormatTagRef("V9.0.0-P01-01"); got != "refs/tags/V9.0.0-P01-01" {
		t.Fatalf("got %q", got)
	}
	if got := FormatTagRef("refs/tags/V9.0.0-P01"); got != "refs/tags/V9.0.0-P01" {
		t.Fatalf("got %q", got)
	}
}

func TestNormalizeRepoBranchForStorage(t *testing.T) {
	if got := NormalizeRepoBranchForStorage("V9.0.0-P01-01"); got != "refs/tags/V9.0.0-P01-01" {
		t.Fatalf("got %q", got)
	}
	if got := NormalizeRepoBranchForStorage("refs/tags/V9.0.0-P01"); got != "refs/tags/V9.0.0-P01" {
		t.Fatalf("got %q", got)
	}
	if got := NormalizeRepoBranchForStorage("release/V9.0.0-P01"); got != "release/V9.0.0-P01" {
		t.Fatalf("got %q", got)
	}
	if got := NormalizeRepoBranchForStorage("develop"); got != "develop" {
		t.Fatalf("got %q", got)
	}
}

func TestCheckoutRefCandidates_tagRef(t *testing.T) {
	got := CheckoutRefCandidates("refs/tags/V9.0.0-P01-01")
	if len(got) < 1 || got[0] != "refs/tags/V9.0.0-P01-01" {
		t.Fatalf("got %v", got)
	}
}

func TestValidateTagName(t *testing.T) {
	if err := ValidateTagName("V9.0.0-P08"); err != nil {
		t.Fatalf("valid tag: %v", err)
	}
	for _, bad := range []string{"", "bad tag", "a..b", ".hidden", "ends.", "a.lock"} {
		if err := ValidateTagName(bad); err == nil {
			t.Fatalf("expected error for %q", bad)
		}
	}
}
