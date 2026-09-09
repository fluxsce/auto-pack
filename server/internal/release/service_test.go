package release

import "testing"

func TestNormalizeStartRequiresBranchAndTag(t *testing.T) {
	_, err := normalizeStart(StartRequest{WorkspaceRecID: "ws"})
	if err == nil {
		t.Fatal("expected error")
	}
	_, err = normalizeStart(StartRequest{WorkspaceRecID: "ws", ReleaseBranch: "release/V9.0.0-P01"})
	if err == nil {
		t.Fatal("expected tag error")
	}
	got, err := normalizeStart(StartRequest{
		WorkspaceRecID: "ws", ReleaseBranch: "release/V9.0.0-P01", TagBranch: "V9.0.0-P01-01",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.ReleaseDate == "" || got.TagBranch != "V9.0.0-P01-01" {
		t.Fatalf("%+v", got)
	}
}

func TestNormalizeStartRejectsBadBranch(t *testing.T) {
	_, err := normalizeStart(StartRequest{
		WorkspaceRecID: "ws", ReleaseBranch: "release/V9 0", TagBranch: "V9.0.0",
	})
	if err == nil {
		t.Fatal("expected invalid branch")
	}
}
