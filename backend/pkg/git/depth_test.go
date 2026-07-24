package git

import (
	"strings"
	"testing"
)

func TestSetGlobalCloneDepth(t *testing.T) {
	prev := GlobalCloneDepth()
	t.Cleanup(func() { SetGlobalCloneDepth(prev) })

	SetGlobalCloneDepth(50)
	if GlobalCloneDepth() != 50 {
		t.Fatalf("got %d want 50", GlobalCloneDepth())
	}
	SetGlobalCloneDepth(0)
	if GlobalCloneDepth() != 0 {
		t.Fatalf("got %d want 0 (full history)", GlobalCloneDepth())
	}
	SetGlobalCloneDepth(-3)
	if GlobalCloneDepth() != DefaultCloneDepth {
		t.Fatalf("got %d want default %d", GlobalCloneDepth(), DefaultCloneDepth)
	}
}

func TestAppendDepthArgs(t *testing.T) {
	prev := GlobalCloneDepth()
	t.Cleanup(func() { SetGlobalCloneDepth(prev) })

	SetGlobalCloneDepth(50)
	got := AppendCloneDepthArgs([]string{"clone"})
	want := []string{"clone", "--depth", "50"}
	if len(got) != len(want) || got[1] != want[1] || got[2] != want[2] {
		t.Fatalf("AppendCloneDepthArgs = %#v want %#v", got, want)
	}
	gotFetch := AppendFetchDepthArgs([]string{"fetch", "origin"})
	if len(gotFetch) != 3 || gotFetch[2] != "--depth=50" {
		t.Fatalf("AppendFetchDepthArgs = %#v", gotFetch)
	}

	SetGlobalCloneDepth(0)
	if len(AppendCloneDepthArgs([]string{"clone"})) != 1 {
		t.Fatal("depth 0 should not append clone --depth")
	}
	if len(AppendFetchDepthArgs([]string{"fetch"})) != 1 {
		t.Fatal("depth 0 should not append fetch --depth")
	}
}

func TestFetchOriginArgs_branchUsesDepth_tagDoesNot(t *testing.T) {
	prev := GlobalCloneDepth()
	t.Cleanup(func() { SetGlobalCloneDepth(prev) })

	SetGlobalCloneDepth(50)
	branchArgs := FetchOriginArgs("origin", "develop")
	hasDepth := false
	for _, a := range branchArgs {
		if a == "--depth=50" {
			hasDepth = true
			break
		}
	}
	if !hasDepth {
		t.Fatalf("branch fetch should include --depth=50: %#v", branchArgs)
	}

	tagArgs := FetchOriginArgs("origin", "refs/tags/V10.0.0-P01")
	for _, a := range tagArgs {
		if strings.HasPrefix(a, "--depth") {
			t.Fatalf("tag fetch must omit --depth for release safety: %#v", tagArgs)
		}
	}
}
