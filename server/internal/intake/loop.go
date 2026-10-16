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
	if s == nil || q == nil || s.mqlog == nil {
		return
	}
	q.SetTraffic(func(ctx context.Context, ev Traffic) {
		if strings.TrimSpace(s.mqEnvID) == "" {
			return
		}
		if err := s.mqlog.Append(ctx, mqlog.Entry{
			EnvRecID:    s.mqEnvID,
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

// Start 订阅通知。已接固定队列时用该队列；否则跟随已启用的 mq 环境。都没有时才轮询目录。
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

// runEnv 每轮读取已启用的 mq 环境。保存或修改队列后下一轮换客户端，不必重启。
func (s *Service) runEnv(ctx context.Context) {
	var held *MNSQueue
	var fp string
	paused := false
	for {
		if ctx.Err() != nil {
			return
		}
		cfg, retries, ok := s.currentMQ(ctx)
		if !ok {
			if held != nil {
				held = nil
				fp = ""
				log.Printf("intake mq env incomplete, pause")
			} else if !paused && mqPartial(cfg) {
				log.Printf("intake mq env incomplete")
			}
			paused = true
			if !waitCtx(ctx, 2*time.Second) {
				return
			}
			continue
		}
		paused = false
		if retries > 0 {
			s.retries = retries
		}
		next := mqFingerprint(cfg)
		if held == nil || next != fp {
			nq, err := OpenMNS(cfg)
			if err != nil {
				log.Printf("intake mns: %v", err)
				if !waitCtx(ctx, 3*time.Second) {
					return
				}
				continue
			}
			s.attachTraffic(nq)
			held = nq
			fp = next
			s.publish = nq.Publish
			log.Printf("intake mns request=%s result=%s", cfg.RequestQueue, cfg.ResultQueue)
		} else if cfg.BatchSize > 0 {
			held.batchSize = cfg.BatchSize
		}
		err := held.poll(ctx, s.handleFromMQ)
		if ctx.Err() != nil {
			return
		}
		if err == nil {
			continue
		}
		wait := held.waitErr()
		if errors.Is(err, errIdle) {
			wait = held.waitIdle()
			if held.waitSeconds == 0 {
				wait = 200 * time.Millisecond
			}
		} else {
			log.Printf("intake mns receive: %v", err)
		}
		if !waitCtx(ctx, wait) {
			return
		}
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
