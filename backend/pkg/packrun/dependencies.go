package packrun

import (
	"fmt"
	"strings"

	"auto-pack/backend/internal/repository"
	"auto-pack/backend/pkg/projectcfg"
)

func (r *Runner) checkBuildDependencies(project *repository.PackProject, cfg projectcfg.Config) error {
	deps := cfg.BuildTriggers.DependsOn
	if len(deps) == 0 {
		return nil
	}
	var missing []string
	var failed []string
	var running []string
	for _, depID := range deps {
		depID = strings.TrimSpace(depID)
		if depID == "" {
			continue
		}
		dep, err := r.projects.FindByRecID(depID)
		if err != nil {
			return fmt.Errorf("dependency project not found: %s", depID)
		}
		if dep.IsEnabled != 1 {
			return fmt.Errorf("dependency project is disabled: %s", depLabel(dep))
		}
		active, err := r.builds.FindRunningByProject(depID)
		if err != nil {
			return err
		}
		if active != nil {
			running = append(running, depLabel(dep))
			continue
		}
		latest, err := r.builds.LatestByProject(depID)
		if err != nil {
			return err
		}
		if latest == nil {
			missing = append(missing, depLabel(dep))
			continue
		}
		if latest.Status != repository.BuildStatusSuccess {
			failed = append(failed, fmt.Sprintf("%s (%s)", depLabel(dep), latest.Status))
		}
	}
	if len(running) > 0 {
		return fmt.Errorf("waiting for upstream build: %s", strings.Join(running, ", "))
	}
	if len(missing) > 0 {
		return fmt.Errorf("upstream never built successfully: %s", strings.Join(missing, ", "))
	}
	if len(failed) > 0 {
		return fmt.Errorf("upstream build not successful: %s", strings.Join(failed, ", "))
	}
	return nil
}

func depLabel(p *repository.PackProject) string {
	return fmt.Sprintf("%s@%s", p.ProjectName, p.ProjectVersion)
}
