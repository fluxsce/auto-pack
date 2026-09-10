package release

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestSyncBroadcastFollowReplaysThenLive(t *testing.T) {
	b := newSyncBroadcast()
	b.emit(SyncEvent{Event: "start", Total: 1})
	b.emit(SyncEvent{Event: "begin", Index: 1, Total: 1, Item: &SyncItem{ProjectName: "a"}})
	go func() {
		time.Sleep(20 * time.Millisecond)
		b.emit(SyncEvent{Event: "item", Index: 1, Total: 1, Item: &SyncItem{ProjectName: "a", Outcome: "synced"}})
		b.finish(SyncResult{Synced: 1}, nil)
	}()
	var evs []string
	res, err := b.follow(context.Background(), func(ev SyncEvent) {
		evs = append(evs, ev.Event)
	})
	if err != nil || res.Synced != 1 {
		t.Fatalf("%+v %v", res, err)
	}
	if got := strings.Join(evs, ","); got != "start,begin,item" {
		t.Fatalf("events %s", got)
	}
}

func TestSyncBroadcastFollowCancel(t *testing.T) {
	b := newSyncBroadcast()
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()
	_, err := b.follow(ctx, nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("got %v", err)
	}
}
