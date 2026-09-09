package runner

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"time"

	"auto-pack/server/internal/job"
	"auto-pack/server/internal/project"
)

const (
	pruneInterval     = time.Hour
	pruneStartupDelay = time.Minute
	pruneBatchLimit   = 200
)

func (e *Engine) pruneLoop(ctx context.Context) {
	timer := time.NewTimer(pruneStartupDelay)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			e.pruneAll(ctx)
			timer.Reset(pruneInterval)
		}
	}
}

func (e *Engine) pruneAll(ctx context.Context) {
	if e.st == nil {
		return
	}
	if !e.pruneMu.TryLock() {
		return
	}
	defer e.pruneMu.Unlock()

	systemDays := 0
	if e.settings != nil {
		systemDays = e.settings.Snapshot(ctx).LogRetentionDays
	}

	targets, err := job.ListRetentionTargets(ctx, e.st)
	if err != nil {
		log.Printf("job prune: list targets: %v", err)
		return
	}
	removed := 0
	for _, t := range targets {
		if ctx.Err() != nil {
			return
		}
		days := retentionDaysOf(t.RecipeJSON, systemDays)
		removed += e.pruneOlderThan(ctx, t.ProjectID, t.WorkspaceID, days)
	}
	if systemDays > 0 {
		workspaces, err := job.ListOrphanWorkspaces(ctx, e.st)
		if err != nil {
			log.Printf("job prune: list orphan workspaces: %v", err)
		} else {
			for _, ws := range workspaces {
				if ctx.Err() != nil {
					return
				}
				removed += e.pruneOlderThan(ctx, "", ws, systemDays)
			}
		}
	}
	if removed > 0 {
		log.Printf("job prune: removed %d expired jobs", removed)
	}
}

func (e *Engine) pruneOlderThan(ctx context.Context, projectID, workspaceID string, days int) int {
	if days <= 0 || e.st == nil {
		return 0
	}
	cutoff := time.Now().UTC().AddDate(0, 0, -days).Format(job.TimeLayout)
	rows, err := job.ExpiredFinished(ctx, e.st, projectID, workspaceID, cutoff, pruneBatchLimit)
	if err != nil || len(rows) == 0 {
		return 0
	}
	ids := make([]string, 0, len(rows))
	for _, row := range rows {
		_ = os.RemoveAll(filepath.Join(e.dataDir, "jobs", row.RecID))
		ids = append(ids, row.RecID)
	}
	n, err := job.DeleteFinishedIDs(ctx, e.st, ids)
	if err != nil {
		log.Printf("job prune: delete %s/%s: %v", workspaceID, projectID, err)
		return 0
	}
	return n
}

func retentionDaysOf(recipeJSON string, systemDays int) int {
	if days := project.ParseRecipe(recipeJSON).LogRetentionDays; days > 0 {
		return days
	}
	return systemDays
}
