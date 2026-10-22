package intake

import (
	"context"
	"errors"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"auto-pack/server/internal/mqlog"
)

// UseMNS 改走固定队列。测试使用；正式进程由 runEnv 按已启用的 mq 环境订阅。
func (s *Service) UseMNS(q *MNSQueue) {
	if s == nil || q == nil {
		return
	}
	s.attachTraffic(q)
	s.mq = q
	s.publish = q.Publish
}

func (s *Service) attachTraffic(q *MNSQueue) {
	s.attachTrafficEnv(q, "")
}

func (s *Service) attachTrafficEnv(q *MNSQueue, envID string) {
	if s == nil || q == nil || s.mqlog == nil {
		return
	}
	q.SetTraffic(func(ctx context.Context, ev Traffic) {
		id := strings.TrimSpace(envID)
		if id == "" {
			id = strings.TrimSpace(s.mqEnvID)
		}
		if id == "" {
			return
		}
		if err := s.mqlog.Append(ctx, mqlog.Entry{
			EnvRecID:    id,
			Direction:   ev.Direction,
			QueueRole:   ev.QueueRole,
			QueueName:   ev.QueueName,
			MessageID:   ev.MessageID,
			Body:        string(ev.Body),
			Source:      mqlog.SourceIntake,
			Deleted:     ev.Deleted,
			ActorUserID: mqlog.ActorIntake,
		}); err != nil {
			log.Printf("intake mqlog: %v", err)
		}
	})
}

// Start 订阅通知。已接固定队列时用该队列；否则跟随全部已启用的 mq 环境。都没有时才轮询目录。
func (s *Service) Start(ctx context.Context) error {
	if s == nil {
		return nil
	}
	if s.mq != nil {
		return s.mq.Listen(s.handleFromMQ)
	}
	runCtx, cancel := context.WithCancel(ctx)
	s.stop = cancel
	if s.envs != nil || s.mqSrc != nil {
		go s.runEnv(runCtx)
		return nil
	}
	if strings.TrimSpace(s.noticeDir) == "" {
		cancel()
		s.stop = nil
		return nil
	}
	go s.loop(runCtx)
	return nil
}

// Stop 停止订阅。正在执行的作业仍由 runner 收尾。
func (s *Service) Stop() {
	if s == nil {
		return
	}
	if s.mq != nil {
		s.mq.Close()
	}
	if s.stop != nil {
		s.stop()
	}
}

// runEnv 每轮读取全部已启用且配置完整的 mq 环境，每条队列单独轮询。
// 保存或修改后下一轮换上对应客户端，不必重启。同一时间只处理一笔通知，回执写回发出它的那条结果队列。
func (s *Service) runEnv(ctx context.Context) {
	live := map[string]*mqSlot{}
	draining := map[string]*mqSlot{}
	var skipLog map[string]struct{}
	defer func() {
		for _, sl := range live {
			sl.halt()
		}
	}()
	for {
		if ctx.Err() != nil {
			return
		}
		reapMQ(draining)
		ready, skipped, err := s.listReadyMQ(ctx)
		if err != nil {
			log.Printf("intake mq env: %v", err)
			if !waitCtx(ctx, 3*time.Second) {
				return
			}
			continue
		}
		skipLog = logSkippedMQ(skipLog, skipped)
		want := make(map[string]mqTarget, len(ready))
		for _, t := range ready {
			want[t.key()] = t
		}
		if len(want) == 0 {
			if len(live) > 0 {
				log.Printf("intake mq env incomplete, pause")
				for id, sl := range live {
					sl.halt()
					draining[id] = sl
				}
				live = map[string]*mqSlot{}
			}
			if !waitCtx(ctx, 2*time.Second) {
				return
			}
			continue
		}
		for id, sl := range live {
			t, ok := want[id]
			if ok && mqFingerprint(t.cfg) == sl.fp {
				continue
			}
			log.Printf("intake mns stop request=%s", sl.q.requestQueue)
			sl.halt()
			draining[id] = sl
			delete(live, id)
		}
		for id, t := range want {
			if _, ok := live[id]; ok {
				continue
			}
			if sl, ok := draining[id]; ok {
				select {
				case <-sl.done:
					delete(draining, id)
				default:
					continue
				}
			}
			nq, err := OpenMNS(t.cfg)
			if err != nil {
				log.Printf("intake mns %s: %v", t.label(), err)
				continue
			}
			s.attachTrafficEnv(nq, t.id)
			sl := newMQSlot(nq, mqFingerprint(t.cfg))
			live[id] = sl
			go s.pollEnv(ctx, sl, t)
			log.Printf("intake mns env=%s request=%s result=%s", t.label(), t.cfg.RequestQueue, t.cfg.ResultQueue)
		}
		if !waitCtx(ctx, 2*time.Second) {
			return
		}
	}
}

// mqSlot 是一条队列的轮询。停掉后要等本轮 poll 结束再开新客户端，避免同一条消息被拉两次。
type mqSlot struct {
	fp      string
	stop    chan struct{}
	done    chan struct{}
	q       *MNSQueue
	stopped bool
}

func newMQSlot(q *MNSQueue, fp string) *mqSlot {
	return &mqSlot{fp: fp, stop: make(chan struct{}), done: make(chan struct{}), q: q}
}

func (sl *mqSlot) halt() {
	if sl == nil || sl.stopped {
		return
	}
	sl.stopped = true
	close(sl.stop)
}

func reapMQ(draining map[string]*mqSlot) {
	for id, sl := range draining {
		select {
		case <-sl.done:
			delete(draining, id)
		default:
		}
	}
}

func logSkippedMQ(prev map[string]struct{}, skipped []string) map[string]struct{} {
	next := make(map[string]struct{}, len(skipped))
	for _, name := range skipped {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		next[name] = struct{}{}
		if _, ok := prev[name]; ok {
			continue
		}
		log.Printf("intake mq env %s incomplete, skip", name)
	}
	return next
}

func (s *Service) pollEnv(ctx context.Context, sl *mqSlot, t mqTarget) {
	defer close(sl.done)
	q := sl.q
	for {
		if ctx.Err() != nil || chanClosed(sl.stop) {
			return
		}
		err := q.poll(ctx, func(hctx context.Context, body []byte) error {
			s.handleMu.Lock()
			defer s.handleMu.Unlock()
			if id := strings.TrimSpace(t.id); id != "" {
				s.mqEnvID = id
			}
			if t.retries > 0 {
				s.retries = t.retries
			}
			s.publish = q.Publish
			return s.handleFromMQ(hctx, body)
		})
		if ctx.Err() != nil || chanClosed(sl.stop) {
			return
		}
		if err == nil {
			continue
		}
		wait := q.waitErr()
		if errors.Is(err, errIdle) {
			wait = q.waitIdle()
			if q.waitSeconds == 0 {
				wait = 200 * time.Millisecond
			}
		} else {
			log.Printf("intake mns receive %s: %v", t.cfg.RequestQueue, err)
		}
		if !waitOrStop(ctx, sl.stop, wait) {
			return
		}
	}
}

func chanClosed(stop <-chan struct{}) bool {
	select {
	case <-stop:
		return true
	default:
		return false
	}
}

func waitOrStop(ctx context.Context, stop <-chan struct{}, d time.Duration) bool {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-stop:
		return false
	case <-timer.C:
		return true
	}
}

func waitCtx(ctx context.Context, d time.Duration) bool {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func (s *Service) loop(ctx context.Context) {
	tick := time.NewTicker(2 * time.Second)
	defer tick.Stop()
	s.drain(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			s.drain(ctx)
		}
	}
}

func (s *Service) drain(ctx context.Context) {
	entries, err := os.ReadDir(s.noticeDir)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Printf("intake notice dir: %v", err)
		}
		return
	}
	for _, ent := range entries {
		if ent.IsDir() || !strings.HasSuffix(strings.ToLower(ent.Name()), ".json") {
			continue
		}
		path := filepath.Join(s.noticeDir, ent.Name())
		raw, err := os.ReadFile(path)
		if err != nil {
			log.Printf("intake read %s: %v", ent.Name(), err)
			continue
		}
		if err := s.Handle(ctx, raw); err != nil {
			log.Printf("intake handle %s: %v", ent.Name(), err)
			continue
		}
		done := filepath.Join(s.noticeDir, "done")
		if mk := os.MkdirAll(done, 0o755); mk != nil {
			log.Printf("intake done dir: %v", mk)
			continue
		}
		dest := filepath.Join(done, ent.Name())
		if ren := os.Rename(path, dest); ren != nil {
			log.Printf("intake archive %s: %v", ent.Name(), ren)
		}
	}
}
