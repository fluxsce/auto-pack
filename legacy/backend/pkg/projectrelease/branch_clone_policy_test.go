package projectrelease

import "testing"

func TestNormalizeExistingTargetPolicy(t *testing.T) {
	cases := []struct {
		in   string
		want ExistingTargetPolicy
	}{
		{"", ExistingTargetOverwrite},
		{"overwrite", ExistingTargetOverwrite},
		{"SKIP", ExistingTargetSkip},
		{"skip_existing", ExistingTargetSkip},
		{"other", ExistingTargetOverwrite},
	}
	for _, tc := range cases {
		if got := NormalizeExistingTargetPolicy(tc.in); got != tc.want {
			t.Fatalf("NormalizeExistingTargetPolicy(%q)=%q want %q", tc.in, got, tc.want)
		}
	}
}

func TestIsProtectedBranchForDelete(t *testing.T) {
	for _, name := range []string{"develop", "Develop", "master", "MAIN", " main "} {
		if !IsProtectedBranchForDelete(name) {
			t.Fatalf("expected protected: %q", name)
		}
	}
	for _, name := range []string{"", "feature/V10", "release/V9.0.0-P01", "develop-backup"} {
		if IsProtectedBranchForDelete(name) {
			t.Fatalf("expected not protected: %q", name)
		}
	}
}

func TestIsStaleForceWithLease(t *testing.T) {
	if !isStaleForceWithLease(errorString("! [rejected] branch -> branch (stale info)")) {
		t.Fatal("expected stale info match")
	}
	if isStaleForceWithLease(errorString("permission denied")) {
		t.Fatal("unexpected match")
	}
}

type errorString string

func (e errorString) Error() string { return string(e) }
