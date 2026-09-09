package packrun

import (
	"os"
	"time"

	"auto-pack/backend/internal/repository"
	"auto-pack/backend/pkg/logger"
	"auto-pack/backend/pkg/projectcfg"
)

// PruneExpiredBuilds removes build records and log files older than the project's retention setting.
func (r *Runner) PruneExpiredBuilds(projectRecID string, retentionDays int) (int, error) {
	retentionDays = projectcfg.EffectiveLogRetentionDays(retentionDays)
	cutoff := time.Now().UTC().AddDate(0, 0, -retentionDays)

	items, err := r.builds.ListOlderThan(projectRecID, cutoff)
	if err != nil {
		return 0, err
	}
	removed := 0
	for _, item := range items {
		if item.Status == repository.BuildStatusPending || item.Status == repository.BuildStatusRunning {
			continue
		}
		if item.LogPath != "" {
			if err := os.Remove(item.LogPath); err != nil && !os.IsNotExist(err) {
				logger.Warn("remove build log failed", "path", item.LogPath, "err", err.Error())
			}
		}
		if err := r.builds.Delete(item.RecID); err != nil {
			return removed, err
		}
		removed++
	}
	return removed, nil
}

// PruneAllProjectBuildLogs runs retention cleanup for every project (e.g. on startup).
func (r *Runner) PruneAllProjectBuildLogs() {
	projects, err := r.projects.List("", "")
	if err != nil {
		logger.Warn("prune build logs: list projects failed", "err", err.Error())
		return
	}
	for _, p := range projects {
		cfg, err := projectcfg.ParseConfigJSON(p.ConfigJSON)
		if err != nil {
			continue
		}
		if n, err := r.PruneExpiredBuilds(p.RecID, cfg.LogRetentionDays); err != nil {
			logger.Warn("prune build logs failed", "project", p.ProjectName, "err", err.Error())
		} else if n > 0 {
			logger.Info("pruned expired build logs", "project", p.ProjectName, "removed", n)
		}
	}
}
