package packrun

import (
	"strings"
	"sync"

	"auto-pack/backend/internal/repository"
	"auto-pack/backend/pkg/logger"
	"auto-pack/backend/pkg/projectcfg"

	"github.com/robfig/cron/v3"
)

// Scheduler runs cron-based builds and reloads jobs on application start.
type Scheduler struct {
	runner   *Runner
	projects *repository.ProjectRepository

	mu      sync.Mutex
	cron    *cron.Cron
	entries map[string]cron.EntryID
}

func NewScheduler(runner *Runner, projects *repository.ProjectRepository) *Scheduler {
	return &Scheduler{
		runner:   runner,
		projects: projects,
		entries:  make(map[string]cron.EntryID),
	}
}

// Start loads all scheduled projects and begins the cron loop.
func (s *Scheduler) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cron != nil {
		s.cron.Stop()
	}
	s.cron = cron.New(cron.WithChain(cron.Recover(cron.DefaultLogger)))
	s.cron.Start()
	return s.reloadLocked()
}

// Stop halts scheduled jobs.
func (s *Scheduler) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cron != nil {
		s.cron.Stop()
		s.cron = nil
	}
	s.entries = make(map[string]cron.EntryID)
}

// Reload refreshes every project's cron registration.
func (s *Scheduler) Reload() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cron == nil {
		return nil
	}
	return s.reloadLocked()
}

// ReloadProject updates cron registration for one project.
func (s *Scheduler) ReloadProject(projectRecID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cron == nil {
		return nil
	}
	projectRecID = strings.TrimSpace(projectRecID)
	if projectRecID == "" {
		return nil
	}
	s.removeLocked(projectRecID)
	p, err := s.projects.FindByRecID(projectRecID)
	if err != nil {
		return nil
	}
	return s.registerLocked(p)
}

func (s *Scheduler) reloadLocked() error {
	for id := range s.entries {
		s.removeLocked(id)
	}
	projects, err := s.projects.List("", "")
	if err != nil {
		return err
	}
	for i := range projects {
		if err := s.registerLocked(&projects[i]); err != nil {
			logger.Warn("schedule register failed",
				"project", projects[i].ProjectName,
				"err", err.Error())
		}
	}
	return nil
}

func (s *Scheduler) registerLocked(p *repository.PackProject) error {
	if p.IsEnabled != 1 {
		return nil
	}
	cfg, err := projectcfg.ParseConfigJSON(p.ConfigJSON)
	if err != nil {
		return err
	}
	sched := cfg.BuildTriggers.Schedule
	if !sched.Enabled {
		return nil
	}
	cronExpr := strings.TrimSpace(sched.Cron)
	if cronExpr == "" {
		return nil
	}
	projectID := p.RecID
	name := p.ProjectName + "@" + p.ProjectVersion
	entryID, err := s.cron.AddFunc(cronExpr, func() {
		if _, err := s.runner.Trigger(projectID, TriggerSchedule); err != nil {
			logger.Warn("scheduled build failed", "project", name, "err", err.Error())
		}
	})
	if err != nil {
		return err
	}
	s.entries[projectID] = entryID
	logger.Info("scheduled build registered", "project", name, "cron", cronExpr)
	return nil
}

func (s *Scheduler) removeLocked(projectRecID string) {
	if entryID, ok := s.entries[projectRecID]; ok {
		s.cron.Remove(entryID)
		delete(s.entries, projectRecID)
	}
}
