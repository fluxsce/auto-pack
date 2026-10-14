package intake

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// UseRocket 改走队列：订阅通知主题，回执按 taskId 写入回执主题。
func (s *Service) UseRocket(q *RocketQueue) {
	if s == nil || q == nil {
		return
	}
	s.mq = q
	s.publish = q.Publish
}

// Start 订阅通知。已接队列时不轮询目录。
func (s *Service) Start(ctx context.Context) error {
	if s == nil {
		return nil
	}
	if s.mq != nil {
		return s.mq.Listen(s.Handle)
	}
	if strings.TrimSpace(s.noticeDir) == "" {
		return nil
	}
	runCtx, cancel := context.WithCancel(ctx)
	s.stop = cancel
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
