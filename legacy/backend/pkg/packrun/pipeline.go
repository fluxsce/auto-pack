package packrun

import (
	"context"
	"strings"

	"auto-pack/backend/pkg/projectcfg"
)

func (r *Runner) triggerDownstream(ctx context.Context, completedProjectID string, buildSucceeded bool) {
	if !buildSucceeded {
		return
	}
	completedProjectID = strings.TrimSpace(completedProjectID)
	if completedProjectID == "" {
		return
	}
	projects, err := r.projects.List("", "")
	if err != nil {
		return
	}
	for i := range projects {
		p := &projects[i]
		if p.IsEnabled != 1 || p.RecID == completedProjectID {
			continue
		}
		cfg, err := projectcfg.ParseConfigJSON(p.ConfigJSON)
		if err != nil || !cfg.BuildTriggers.HasDependency(completedProjectID) {
			continue
		}
		if err := r.checkBuildDependencies(p, cfg); err != nil {
			continue
		}
		if _, err := r.Trigger(p.RecID, TriggerPipeline); err != nil {
			_ = err
		}
	}
	_ = ctx
}
