package release

import (
	"errors"
	"testing"
)

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

func TestBeginSyncRejectsSameWorkspace(t *testing.T) {
	svc := &Service{}
	if err := svc.beginSync("ws-a"); err != nil {
		t.Fatal(err)
	}
	if err := svc.beginSync("ws-a"); !errors.Is(err, ErrSyncInProgress) {
		t.Fatalf("want in progress, got %v", err)
	}
	if err := svc.beginSync("ws-b"); err != nil {
		t.Fatal(err)
	}
	svc.endSync("ws-a")
	if err := svc.beginSync("ws-a"); err != nil {
		t.Fatal(err)
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
