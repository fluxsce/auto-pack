package batchrun

import (
	"context"
	"fmt"
	"time"

	"auto-pack/server/internal/project"
	ord "auto-pack/server/internal/release/order"
	"auto-pack/server/internal/settings"
)

// BuildFn 跑单个工程。调用方传入 RunBuild，测试可注入。
type BuildFn func(ctx context.Context, prj project.Project) Outcome

// GraphHooks 批次日志与进度。OnLog / OnStep 可能并发，调用方应自行串行写盘。
type GraphHooks struct {
	// MaxInflight 同时 wait 的工程数，0 则用系统默认构建并发；上限与打包队列相同。
	MaxInflight int
	OnParallel  func(started []project.Project)
	OnStart     func(prj project.Project)
	OnSkip      func(prj project.Project, reason string)
	OnFinish    func(prj project.Project, out Outcome, took time.Duration)
	OnCancel    func(prj project.Project, err error)
}

func graphInflightLimit(n int) int {
	if n <= 0 {
		n = settings.DefaultJobConcurrency
	}
	if n > settings.MaxJobConcurrency {
		return settings.MaxJobConcurrency
	}
	return n
}

func callSafe(fn func()) {
	if fn == nil {
		return
	}
	defer func() { _ = recover() }()
	fn()
}

// RunGraph 按分析图启动：队列内上游成功后才编下游；上游失败则跳过，避免缺包。
// 无互相依赖的工程并行。循环依赖在并行排空后按原队列顺序补跑。
func RunGraph(ctx context.Context, queue []project.Project, analysis ord.MavenAnalysis, build BuildFn, hooks GraphHooks) map[string]string {
	if build == nil {
		out := map[string]string{}
		for _, p := range queue {
			out[p.RecID] = ord.OutcomeFailed
		}
		return out
	}
	if ctx == nil {
		ctx = context.Background()
	}
	gate := ord.NewGate(queue, analysis)
	done := map[string]string{}
	started := map[string]bool{}
	limit := graphInflightLimit(hooks.MaxInflight)
	type fin struct {
		prj  project.Project
		out  Outcome
		took time.Duration
	}
	results := make(chan fin, limit)
	inflight := 0

	startOne := func(p project.Project) {
		started[p.RecID] = true
		inflight++
		callSafe(func() {
			if hooks.OnStart != nil {
				hooks.OnStart(p)
			}
		})
		go func(p project.Project) {
			t0 := time.Now()
			var out Outcome
			func() {
				defer func() {
					if rec := recover(); rec != nil {
						out = Outcome{Message: fmt.Sprintf("panic: %v", rec)}
					}
				}()
				out = build(ctx, p)
			}()
			results <- fin{prj: p, out: out, took: time.Since(t0)}
		}(p)
	}

	kick := func() {
		if ctx.Err() != nil {
			for _, p := range queue {
				if started[p.RecID] || done[p.RecID] != "" {
					continue
				}
				done[p.RecID] = ord.OutcomeFailed
				callSafe(func() {
					if hooks.OnCancel != nil {
						hooks.OnCancel(p, ctx.Err())
					}
				})
			}
			return
		}
		var ready []project.Project
		for _, p := range queue {
			if started[p.RecID] || done[p.RecID] != "" {
				continue
			}
			ok, skip := gate.Decide(p.RecID, done)
			if skip != "" {
				done[p.RecID] = ord.OutcomeSkipped
				callSafe(func() {
					if hooks.OnSkip != nil {
						hooks.OnSkip(p, skip)
					}
				})
				continue
			}
			if ok {
				ready = append(ready, p)
			}
		}
		room := limit - inflight
		if room < 0 {
			room = 0
		}
		if room < len(ready) {
			ready = ready[:room]
		}
		if len(ready) > 1 {
			callSafe(func() {
				if hooks.OnParallel != nil {
					hooks.OnParallel(ready)
				}
			})
		}
		for _, p := range ready {
			startOne(p)
		}
	}

	kick()
	for inflight > 0 {
		ev := <-results
		inflight--
		if ev.out.OK {
			done[ev.prj.RecID] = ord.OutcomeOK
		} else {
			done[ev.prj.RecID] = ord.OutcomeFailed
		}
		callSafe(func() {
			if hooks.OnFinish != nil {
				hooks.OnFinish(ev.prj, ev.out, ev.took)
			}
		})
		kick()
	}

	for _, p := range queue {
		if done[p.RecID] != "" || started[p.RecID] {
			continue
		}
		_, skip := gate.Decide(p.RecID, done)
		if skip != "" {
			done[p.RecID] = ord.OutcomeSkipped
			callSafe(func() {
				if hooks.OnSkip != nil {
					hooks.OnSkip(p, skip)
				}
			})
			continue
		}
		if ctx.Err() != nil {
			done[p.RecID] = ord.OutcomeFailed
			callSafe(func() {
				if hooks.OnCancel != nil {
					hooks.OnCancel(p, ctx.Err())
				}
			})
			continue
		}
		callSafe(func() {
			if hooks.OnStart != nil {
				hooks.OnStart(p)
			}
		})
		started[p.RecID] = true
		t0 := time.Now()
		out := build(ctx, p)
		if out.OK {
			done[p.RecID] = ord.OutcomeOK
		} else {
			done[p.RecID] = ord.OutcomeFailed
		}
		callSafe(func() {
			if hooks.OnFinish != nil {
				hooks.OnFinish(p, out, time.Since(t0))
			}
		})
	}
	return done
}
