package packrun

import (
	"context"
	"fmt"
	"strings"

	"auto-pack/backend/internal/repository"
	"auto-pack/backend/pkg/maven"
	"auto-pack/backend/pkg/mavendeps"
	"auto-pack/backend/pkg/projectfs"
)

func (r *Runner) ensureMavenPrerequisites(
	ctx context.Context,
	project *repository.PackProject,
	ws *repository.PackWorkspace,
	targetWorkDir string,
	log *buildLogger,
) error {
	if strings.ToLower(strings.TrimSpace(project.ProjectType)) != "maven" {
		return nil
	}

	log.section("Maven dependency analysis")
	settingsPath := r.mavenSettingsPath(project)
	mavenHome := r.mavenHome(project)
	localRepo := r.mavenLocalRepository(project)
	if localRepo != "" {
		log.step("Local repository: %s", localRepo)
	}
	if settingsPath != "" {
		if raw := strings.TrimSpace(mavendeps.RawLocalRepositoryFromSettings(settingsPath)); raw != "" &&
			mavendeps.IsPlaceholderLocalRepositoryPath(strings.TrimSpace(raw)) {
			log.step("Ignored placeholder localRepository in settings.xml; using Maven local repository under MAVEN_HOME")
		}
	}

	entries, err := r.mavenScanEntries(project.WorkspaceRecID, ws)
	if err != nil {
		return err
	}
	wsIndex, scanMeta, err := mavendeps.ScanModulesWithMavenHome(entries, localRepo, mavenHome)
	if err != nil {
		return err
	}
	for _, msg := range scanMeta.MissingPOM {
		log.step("Skip (no pom): %s", msg)
	}
	for _, line := range wsIndex.DependencyDiagnostics(project.RecID) {
		log.step("%s", line)
	}
	analysis, err := wsIndex.AnalyzeTarget(project.RecID)
	if err != nil {
		log.step("Maven dependency analysis skipped: %v", err)
		return nil
	}
	for _, m := range analysis.SkippedInstalled {
		log.step("Already in local repo, skip install: %s (%s)", m.ProjectName, m.Coordinates.Key())
	}

	if len(analysis.BuildOrder) == 0 {
		log.step("No workspace upstream modules require install.")
	} else {
		log.step("Upstream install order (%d module(s)):", len(analysis.BuildOrder))
		for i, m := range analysis.BuildOrder {
			log.step("  %d. %s %s", i+1, m.ProjectName, m.Coordinates.Key()+"@"+m.Coordinates.Version)
		}

		installCmd := mavendeps.DefaultInstallCmd
		if settingsPath != "" {
			installCmd = maven.InjectSettingsIntoScript(installCmd, settingsPath)
		}

		if err := r.installUpstreamModules(ctx, analysis, ws, installCmd, settingsPath, log); err != nil {
			return err
		}
	}

	unsatisfied := wsIndex.ValidateTargetDependencies(project.RecID)
	if len(unsatisfied) > 0 {
		log.step("Dependency validation failed (%d unsatisfied):", len(unsatisfied))
		for _, u := range unsatisfied {
			log.step("  - %s", u.Reason)
		}
		return fmt.Errorf("maven dependencies not satisfied: %s", unsatisfied[0].Reason)
	}
	log.step("All direct dependencies satisfied in local repository.")

	_ = targetWorkDir
	return nil
}

func (r *Runner) installUpstreamModules(
	ctx context.Context,
	analysis *mavendeps.AnalysisResult,
	ws *repository.PackWorkspace,
	installCmd, settingsPath string,
	log *buildLogger,
) error {
	if analysis == nil || len(analysis.BuildOrder) == 0 {
		return nil
	}
	cyclic := len(analysis.CyclicGroups) > 0
	maxPasses := 1
	if cyclic {
		maxPasses = len(analysis.BuildOrder)
		if maxPasses < 2 {
			maxPasses = 2
		}
		if maxPasses > 4 {
			maxPasses = 4
		}
		log.step("Cyclic workspace dependencies detected; upstream install will retry up to %d pass(es)", maxPasses)
	}

	pending := append([]mavendeps.Module(nil), analysis.BuildOrder...)
	for pass := 1; pass <= maxPasses && len(pending) > 0; pass++ {
		if cyclic && pass > 1 {
			log.step("Upstream install retry pass %d/%d (%d module(s))", pass, maxPasses, len(pending))
		}
		var stillPending []mavendeps.Module
		for _, mod := range pending {
			if err := r.installUpstreamModule(ctx, mod, ws, installCmd, settingsPath, log); err != nil {
				if cyclic {
					log.step("Upstream install deferred (cycle): %s — %v", mod.ProjectName, err)
					stillPending = append(stillPending, mod)
					continue
				}
				return err
			}
		}
		pending = stillPending
	}
	if len(pending) > 0 {
		return fmt.Errorf("upstream install failed after cyclic retries: %s", pending[0].ProjectName)
	}
	return nil
}

func (r *Runner) installUpstreamModule(
	ctx context.Context,
	mod mavendeps.Module,
	ws *repository.PackWorkspace,
	installCmd, settingsPath string,
	log *buildLogger,
) error {
	release := r.acquireProjectBuildWithLog(
		mod.ProjectRecID,
		mod.ProjectName,
		log.step,
		log.step,
	)
	defer release()

	up, err := r.projects.FindByRecID(mod.ProjectRecID)
	if err != nil {
		return err
	}
	if up.IsEnabled != 1 {
		return fmt.Errorf("upstream project disabled: %s", mod.ProjectName)
	}
	upDir := projectfs.ResolveProjectAbsDir(up, ws, r.dataDir)
	if err := r.syncSource(ctx, up, upDir, log, &syncSourceOpts{compact: true}); err != nil {
		return fmt.Errorf("sync upstream %s: %w", mod.ProjectName, err)
	}
	log.step("Installing upstream module: %s", mod.ProjectName)
	shellEnv, err := r.buildShellEnv(up)
	if err != nil {
		return err
	}
	cmd := installCmd
	if sp := r.mavenSettingsPath(up); sp != "" && sp != settingsPath {
		cmd = maven.InjectSettingsIntoScript(mavendeps.DefaultInstallCmd, sp)
	}
	res, err := log.runShell(ctx, upDir, shellEnv, cmd)
	if err != nil {
		return fmt.Errorf("install %s: %w", mod.ProjectName, err)
	}
	if res.ExitCode != 0 {
		return fmt.Errorf("install %s: exit %d", mod.ProjectName, res.ExitCode)
	}
	log.step("Upstream install completed: %s", mod.ProjectName)
	return nil
}

func (r *Runner) mavenScanEntries(workspaceRecID string, ws *repository.PackWorkspace) ([]mavendeps.ScanEntry, error) {
	projects, err := r.projects.List(workspaceRecID, "maven")
	if err != nil {
		return nil, err
	}
	out := make([]mavendeps.ScanEntry, 0, len(projects))
	for i := range projects {
		p := &projects[i]
		if p.IsEnabled != 1 {
			continue
		}
		dir := projectfs.ResolveProjectAbsDir(p, ws, r.dataDir)
		out = append(out, mavendeps.ScanEntry{
			ProjectRecID:   p.RecID,
			ProjectName:    p.ProjectName,
			ProjectVersion: p.ProjectVersion,
			ProjectDir:     dir,
		})
	}
	return out, nil
}

func (r *Runner) scanMavenWorkspaceIndex(
	workspaceRecID, settingsXML string,
) (*repository.PackWorkspace, *mavendeps.Workspace, *mavendeps.AnalysisResult, error) {
	ws, err := r.workspaces.FindByRecID(workspaceRecID)
	if err != nil {
		return nil, nil, nil, err
	}
	localRepo := mavendeps.ResolveLocalRepository(maven.ResolveSettingsPath(settingsXML), "")
	entries, err := r.mavenScanEntries(workspaceRecID, ws)
	if err != nil {
		return nil, nil, nil, err
	}
	wsIndex, scanMeta, err := mavendeps.ScanModules(entries, localRepo)
	if err != nil {
		return nil, nil, nil, err
	}
	return ws, wsIndex, scanMeta, nil
}

// AnalyzeMavenWorkspace exposes single-project dependency analysis for API handlers.
func (r *Runner) AnalyzeMavenWorkspace(workspaceRecID, targetRecID, settingsXML string) (*mavendeps.AnalysisResult, []mavendeps.Module, error) {
	_, wsIndex, scanMeta, err := r.scanMavenWorkspaceIndex(workspaceRecID, settingsXML)
	if err != nil {
		return nil, nil, err
	}
	analysis, err := wsIndex.AnalyzeTarget(targetRecID)
	if err != nil {
		return nil, wsIndex.Modules(), err
	}
	if scanMeta != nil {
		analysis.MissingPOM = append(analysis.MissingPOM, scanMeta.MissingPOM...)
		analysis.Warnings = append(analysis.Warnings, scanMeta.Warnings...)
	}
	return analysis, wsIndex.Modules(), nil
}

// AnalyzeMavenWorkspaceAll returns full-workspace build order and internal dependency edges.
func (r *Runner) AnalyzeMavenWorkspaceAll(workspaceRecID, settingsXML string) (*mavendeps.WorkspaceAnalysisResult, []mavendeps.Module, error) {
	_, wsIndex, scanMeta, err := r.scanMavenWorkspaceIndex(workspaceRecID, settingsXML)
	if err != nil {
		return nil, nil, err
	}
	analysis, err := wsIndex.AnalyzeAll()
	if err != nil {
		return nil, wsIndex.Modules(), err
	}
	if scanMeta != nil {
		analysis.MissingPOM = append(analysis.MissingPOM, scanMeta.MissingPOM...)
		analysis.Warnings = append(analysis.Warnings, scanMeta.Warnings...)
	}
	return analysis, wsIndex.Modules(), nil
}
