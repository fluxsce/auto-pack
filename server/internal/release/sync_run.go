package release

import (
	"context"
	"sync"
)

// syncBroadcast 保存一次预同步的事件，后来的请求回放已有进度并继续跟到结束。
type syncBroadcast struct {
	mu     sync.Mutex
	events []SyncEvent
	wait   chan struct{}
	done   bool
	result SyncResult
	err    error
}

func newSyncBroadcast() *syncBroadcast {
	return &syncBroadcast{wait: make(chan struct{})}
}

func (b *syncBroadcast) emit(ev SyncEvent) {
	if ev.Item != nil {
		cp := *ev.Item
		ev.Item = &cp
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.events = append(b.events, ev)
	b.wakeLocked()
}

func (b *syncBroadcast) finish(res SyncResult, err error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.result = res
	b.err = err
	b.done = true
	b.wakeLocked()
}

func (b *syncBroadcast) wakeLocked() {
	close(b.wait)
	b.wait = make(chan struct{})
}

func (b *syncBroadcast) follow(ctx context.Context, progress SyncListener) (SyncResult, error) {
	if b == nil {
		return SyncResult{}, ErrSyncInProgress
	}
	idx := 0
	for {
		if err := ctx.Err(); err != nil {
			return SyncResult{}, err
		}
		b.mu.Lock()
		for idx < len(b.events) {
			ev := b.events[idx]
			idx++
			b.mu.Unlock()
			if progress != nil {
				progress(ev)
			}
			if err := ctx.Err(); err != nil {
				return SyncResult{}, err
			}
			b.mu.Lock()
		}
		if b.done {
			res, err := b.result, b.err
			b.mu.Unlock()
			return res, err
		}
		wait := b.wait
		b.mu.Unlock()
		select {
		case <-ctx.Done():
			return SyncResult{}, ctx.Err()
		case <-wait:
		}
	}
}
