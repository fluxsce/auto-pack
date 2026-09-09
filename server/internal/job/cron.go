package job

import (
	"context"
	"log"
	"strings"

	"auto-pack/server/internal/project"

	"github.com/robfig/cron/v3"
)

// Scheduler 只把工程 cron_expr 转成入队，不跑构建。
type Scheduler struct {
	cron *cron.Cron
	svc  *Service
	prj  *project.Service
}

// NewScheduler returns a cron scheduler.
func NewScheduler(svc *Service, prj *project.Service) *Scheduler {
	return &Scheduler{cron: cron.New(), svc: svc, prj: prj}
}

// Start loads enabled project crons.
func (s *Scheduler) Start(ctx context.Context) error {
	list, err := s.prj.ListEnabledCron(ctx)
	if err != nil {
		return err
	}
	for _, p := range list {
		expr := strings.TrimSpace(p.CronExpr)
		if expr == "" {
			continue
		}
		p := p
		_, err := s.cron.AddFunc(expr, func() {
			_, err := s.svc.SubmitSystem(context.Background(), SubmitRequest{
				WorkspaceRecID: p.WorkspaceRecID,
				ProjectRecID:   p.RecID,
				JobType:        TypeBuild,
			})
			if err != nil {
				log.Printf("cron enqueue %s: %v", p.ProjectName, err)
			}
		})
		if err != nil {
			log.Printf("cron invalid %s %s: %v", p.ProjectName, expr, err)
		}
	}
	s.cron.Start()
	return nil
}

// Stop stops the cron runner.
func (s *Scheduler) Stop() {
	if s.cron != nil {
		s.cron.Stop()
	}
}

// Reload rebuilds entries from the database.
func (s *Scheduler) Reload(ctx context.Context) error {
	s.Stop()
	s.cron = cron.New()
	return s.Start(ctx)
}
