package batchlock

import (
	"errors"
	"testing"
)

func TestGatePerWorkspace(t *testing.T) {
	g := New()
	if err := g.Try("ws-a", KindRelease); err != nil {
		t.Fatal(err)
	}
	if err := g.Try("ws-a", KindBuild); !errors.Is(err, ErrBusy) {
		t.Fatalf("same workspace should reject, got %v", err)
	}
	var busy BusyError
	if !errors.As(g.Try("ws-a", KindBuild), &busy) || busy.Kind != KindRelease {
		t.Fatalf("want release busy, got %+v", busy)
	}
	if err := g.Try("ws-b", KindBuild); err != nil {
		t.Fatalf("other workspace should start, got %v", err)
	}
	g.Done("ws-a")
	if err := g.Try("ws-a", KindBuild); err != nil {
		t.Fatal(err)
	}
}
