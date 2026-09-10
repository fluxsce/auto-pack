package release

import "testing"

func TestSyncedReady(t *testing.T) {
	res := SyncResult{Items: []SyncItem{
		{ProjectName: "ok", Outcome: "synced"},
		{ProjectName: "bad", Outcome: "failed"},
		{ProjectName: "local", Outcome: "skipped"},
	}}
	if !res.SyncedReady("ok") {
		t.Fatal("synced")
	}
	if res.SyncedReady("bad") || res.SyncedReady("local") || res.SyncedReady("missing") || res.SyncedReady("") {
		t.Fatal("only successful fetch may skip job sync")
	}
}
