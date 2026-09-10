// Package batchlock 按工作空间互斥自动构建与自动发版。
package batchlock

import (
	"errors"
	"strings"
	"sync"
)

const (
	// KindRelease 表示该空间正在自动发版（含发版后构建/推送）。
	KindRelease Kind = "release"
	// KindBuild 表示该空间正在自动构建。
	KindBuild Kind = "build"
)

// Kind 是占用空间的批次类型。
type Kind string

// ErrBusy 同一空间已有构建或发版在跑。
var ErrBusy = errors.New("workspace batch already in progress")

// BusyError 说明谁占用了该空间。
type BusyError struct {
	Kind Kind
}

func (e BusyError) Error() string {
	switch e.Kind {
	case KindRelease:
		return "workspace auto-release already in progress"
	case KindBuild:
		return "workspace auto-build already in progress"
	default:
		return ErrBusy.Error()
	}
}

func (e BusyError) Unwrap() error { return ErrBusy }

// Gate 按工作空间单飞：构建与发版不能同时占同一空间。
type Gate struct {
	mu     sync.Mutex
	active map[string]Kind
}

// New 构造空闲 Gate。
func New() *Gate {
	return &Gate{active: map[string]Kind{}}
}

// Try 占用空间。已被占用时返回 BusyError。
func (g *Gate) Try(workspaceID string, kind Kind) error {
	id := strings.TrimSpace(workspaceID)
	if id == "" {
		return errors.New("workspace_rec_id is required")
	}
	if g == nil {
		return nil
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.active == nil {
		g.active = map[string]Kind{}
	}
	if existing, ok := g.active[id]; ok {
		return BusyError{Kind: existing}
	}
	g.active[id] = kind
	return nil
}

// Active 返回该空间当前占用类型，空闲则为空。
func (g *Gate) Active(workspaceID string) Kind {
	if g == nil {
		return ""
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.active[strings.TrimSpace(workspaceID)]
}

// Done 释放空间占用。
func (g *Gate) Done(workspaceID string) {
	if g == nil {
		return
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	delete(g.active, strings.TrimSpace(workspaceID))
}
