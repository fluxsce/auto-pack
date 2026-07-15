package packrun

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"auto-pack/backend/internal/repository"
	"auto-pack/backend/pkg/envtypes"
	"auto-pack/backend/pkg/git"
	"auto-pack/backend/pkg/logger"
	"auto-pack/backend/pkg/maven"
	"auto-pack/backend/pkg/nodejs"
	"auto-pack/backend/pkg/projectcfg"
	"auto-pack/backend/pkg/projectfs"
	"auto-pack/backend/pkg/redact"
	"auto-pack/backend/pkg/shell"
)

func (r *Runner) executeBuild(buildRecID string, mode TriggerMode) {
	ctx := context.Background()
	build, err := r.builds.FindByRecID(buildRecID)
	if err != nil {
		return
	}
	project, err := r.projects.FindByRecID(build.ProjectRecID)
	if err != nil {
		_ = r.builds.UpdateStatus(buildRecID, repository.BuildStatusFailed, err.Error(), nil, timePtr(time.Now().UTC()))
		return
	}
	ws, err := r.workspaces.FindByRecID(project.WorkspaceRecID)
	if err != nil {
		_ = r.builds.UpdateStatus(buildRecID, repository.BuildStatusFailed, err.Error(), nil, timePtr(time.Now().UTC()))
		return
	}

	logFile, err := os.OpenFile(build.LogPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		_ = r.builds.UpdateStatus(buildRecID, repository.BuildStatusFailed, err.Error(), nil, timePtr(time.Now().UTC()))
		return
	}
	defer logFile.Close()

	log := newBuildLogger(logFile)
	now := time.Now().UTC()
	_ = r.builds.UpdateStatus(buildRecID, repository.BuildStatusRunning, "running", &now, nil)

	workDir := projectfs.ResolveProjectSourceAbsDir(project, ws, r.dataDir)
	log.header(project.ProjectName, build.BuildNum, workDir)
	log.line("Trigger: %s", mode.String())
	log.line("Build started at %s", logTimestamp())

	status := repository.BuildStatusSuccess
	message := "build finished successfully"
	finished := time.Now().UTC()

	releaseSync := r.acquireProjectBuild(project.RecID)
	if err := r.syncSource(ctx, project, workDir, log, nil); err != nil {
		releaseSync()
		status = repository.BuildStatusFailed
		message = redact.Redact(err.Error())
		log.finished(status, message)
		_ = r.builds.UpdateStatus(buildRecID, status, message, nil, &finished)
		return
	}
	releaseSync()

	// Do not hold this project's lock while installing workspace upstream modules;
	// otherwise parallel pipeline builds on circular Maven deps deadlock forever.
	if err := r.ensureMavenPrerequisites(ctx, project, ws, workDir, log); err != nil {
		status = repository.BuildStatusFailed
		message = redact.Redact(err.Error())
		log.finished(status, message)
		_ = r.builds.UpdateStatus(buildRecID, status, message, nil, &finished)
		return
	}

	releaseSelf := r.acquireProjectBuildWithLog(project.RecID, project.ProjectName, log.step, log.step)
	defer releaseSelf()

	cfg, err := projectcfg.ParseConfigJSON(project.ConfigJSON)
	if err != nil {
		status = repository.BuildStatusFailed
		message = err.Error()
		log.finished(status, message)
		_ = r.builds.UpdateStatus(buildRecID, status, message, nil, &finished)
		return
	}
	defer func() {
		if _, perr := r.PruneExpiredBuilds(project.RecID, cfg.LogRetentionDays); perr != nil {
			logger.Warn("prune build logs after build failed", "project", project.ProjectName, "err", perr.Error())
		}
	}()
	buildCmd := strings.TrimSpace(cfg.BuildCmd)
	if buildCmd == "" {
		log.section("Build")
		log.step("No build command configured in project settings; skipping build step.")
	} else {
		log.section("Build")
		if err := r.logToolchain(project, log); err != nil {
			status = repository.BuildStatusFailed
			message = err.Error()
			log.finished(status, message)
			_ = r.builds.UpdateStatus(buildRecID, status, message, nil, &finished)
			return
		}

		shellEnv, err := r.buildShellEnv(project)
		if err != nil {
			status = repository.BuildStatusFailed
			message = err.Error()
			log.finished(status, message)
			_ = r.builds.UpdateStatus(buildRecID, status, message, nil, &finished)
			return
		}
		npmrcPath, cleanupNpmrc, npmrcSummary, npmrcErr := r.prepareNodeNpmUserConfig(project)
		if npmrcErr != nil {
			status = repository.BuildStatusFailed
			message = npmrcErr.Error()
			log.finished(status, message)
			_ = r.builds.UpdateStatus(buildRecID, status, message, nil, &finished)
			return
		}
		if cleanupNpmrc != nil {
			defer cleanupNpmrc()
		}
		if npmrcPath != "" {
			shellEnv = append(shellEnv, "NPM_CONFIG_USERCONFIG="+npmrcPath)
			if npmrcSummary != "" {
				log.step("npm/pnpm userconfig: %s", npmrcSummary)
			} else {
				log.step("npm/pnpm userconfig: %s", npmrcPath)
			}
		}
		nodeCfg := r.nodejsConfig(project)
		if err := os.MkdirAll(workDir, 0o755); err != nil {
			status = repository.BuildStatusFailed
			message = fmt.Sprintf("mkdir work dir: %v", err)
			log.finished(status, message)
			_ = r.builds.UpdateStatus(buildRecID, status, message, nil, &finished)
			return
		}
		if nodejs.PnpmAllowAllBuildsEnabled(nodeCfg) {
			shellEnv = nodejs.AppendPnpmAllowAllBuildsEnv(shellEnv)
			buildCmd = nodejs.InjectPnpmAllowAllBuilds(buildCmd)
			if err := nodejs.EnsureWorkdirPnpmAllowBuilds(workDir); err != nil {
				status = repository.BuildStatusFailed
				message = err.Error()
				log.finished(status, message)
				_ = r.builds.UpdateStatus(buildRecID, status, message, nil, &finished)
				return
			}
			log.step("pnpm: allow all builds (env + CLI + project .npmrc)")
		}

		if settingsPath := r.mavenSettingsPath(project); settingsPath != "" {
			buildCmd = maven.InjectSettingsIntoScript(buildCmd, settingsPath)
			log.step("Maven settings.xml: %s", settingsPath)
		}

		log.step("Execute shell")
		res, err := log.runShell(ctx, workDir, shellEnv, buildCmd)
		if err != nil {
			status = repository.BuildStatusFailed
			message = err.Error()
		} else if res.ExitCode != 0 {
			status = repository.BuildStatusFailed
			message = fmt.Sprintf("command exited with code %d", res.ExitCode)
		} else {
			log.step("Build step completed successfully (exit 0).")
		}

		if strings.TrimSpace(cfg.OutputDir) != "" {
			log.step("Expected output directory: %s", cfg.OutputDir)
		}
		if strings.TrimSpace(cfg.ArtifactPattern) != "" {
			log.step("Artifact pattern: %s", cfg.ArtifactPattern)
		}
	}

	buildSucceeded := status == repository.BuildStatusSuccess
	if err := r.runPostBuildActions(ctx, project, cfg, workDir, build, buildSucceeded, message, log); err != nil {
		status = repository.BuildStatusFailed
		message = err.Error()
	}

	log.finished(status, message)
	_ = r.builds.UpdateStatus(buildRecID, status, message, nil, &finished)

	go r.triggerDownstream(context.Background(), project.RecID, buildSucceeded)
}

func (r *Runner) logToolchain(project *repository.PackProject, log *buildLogger) error {
	log.step("Preparing toolchain environment...")
	type ref struct {
		recID string
		typ   string
	}
	refs := []ref{
		{project.JdkEnvRecID, envtypes.TypeJDK},
		{project.MavenEnvRecID, envtypes.TypeMaven},
		{project.NodejsEnvRecID, envtypes.TypeNodeJS},
	}
	for _, ref := range refs {
		if ref.recID == "" {
			continue
		}
		item, err := r.env.FindByRecID(ref.recID)
		if err != nil {
			return fmt.Errorf("%s environment: %w", ref.typ, err)
		}
		log.step("Using %s environment: %s (%s)", ref.typ, item.EnvName, item.RecID)
		switch ref.typ {
		case envtypes.TypeJDK:
			var cfg envtypes.JDKConfig
			if err := json.Unmarshal([]byte(item.ConfigJSON), &cfg); err != nil {
				return err
			}
			if home := strings.TrimSpace(cfg.JavaHome); home != "" {
				log.step("JAVA_HOME=%s", home)
			}
		case envtypes.TypeMaven:
			var cfg envtypes.MavenConfig
			if err := json.Unmarshal([]byte(item.ConfigJSON), &cfg); err != nil {
				return err
			}
			if home := strings.TrimSpace(cfg.MavenHome); home != "" {
				log.step("MAVEN_HOME=%s", home)
			}
			if settings := maven.ResolveSettingsPath(cfg.SettingsXML); settings != "" {
				log.step("settings.xml=%s", settings)
			} else if strings.TrimSpace(cfg.SettingsXML) != "" {
				log.step("settings.xml not found: %s", strings.TrimSpace(cfg.SettingsXML))
			}
		case envtypes.TypeNodeJS:
			var cfg envtypes.NodeJSConfig
			if err := json.Unmarshal([]byte(item.ConfigJSON), &cfg); err != nil {
				return err
			}
			if p := strings.TrimSpace(cfg.NodePath); p != "" {
				log.step("Node.js: %s", p)
			}
			if p := nodejs.ResolveNpmrcPath(cfg.NpmrcPath); p != "" {
				log.step("npmrc: %s", p)
			}
			if reg := strings.TrimSpace(cfg.Registry); reg != "" {
				log.step("npm registry: %s", reg)
			}
			if nodejs.PnpmAllowAllBuildsEnabled(cfg) {
				log.step("pnpm: allow all dependency build scripts")
			}
		}
	}
	return nil
}

func (r *Runner) syncSource(ctx context.Context, project *repository.PackProject, workDir string, log *buildLogger, opts *syncSourceOpts) error {
	var o syncSourceOpts
	if opts != nil {
		o = *opts
	}
	repoURL := strings.TrimSpace(project.RepoURL)
	if !projectcfg.UsesGitSource(project) {
		if !o.compact {
			log.section("Source")
			if repoURL != "" {
				log.step("Local source project; using local directory (remote push only, no pull).")
			} else {
				log.step("Using local source directory.")
			}
		}
		if _, err := os.Stat(workDir); err != nil {
			return fmt.Errorf("local source path: %w", err)
		}
		return nil
	}
	if repoURL == "" {
		if !o.compact {
			log.section("Source")
			log.step("No Git repository URL configured; using local workspace only.")
		}
		return nil
	}

	if !o.compact {
		log.section("Source")
		log.step("Fetching changes from the remote Git repository")
	}

	gitEnv, shellEnv, cloneURL, authLabel, err := r.gitShellEnv(project, repoURL)
	if err != nil {
		return err
	}
	_ = gitEnv
	if authLabel != "" && !o.compact {
		log.step("Using Git credentials: %s", authLabel)
	}
	displayURL := sanitizeURLForLog(repoURL)
	branch := strings.TrimSpace(project.RepoBranch)
	if branch == "" {
		branch = "main"
	}

	if err := ensureProjectParentDir(workDir); err != nil {
		return fmt.Errorf("prepare project directory: %w", err)
	}

	if !o.compact {
		if _, err := log.runGit(ctx, "", shellEnv, "--version"); err != nil {
			return err
		}
	}

	if git.IsRepository(ctx, workDir) && !git.HasResolvableHEAD(ctx, workDir, shellEnv) {
		if !o.compact {
			log.step("Removing incomplete local repository (interrupted sync)")
		}
		_ = os.RemoveAll(workDir)
	}

	if git.IsRepository(ctx, workDir) {
		err := r.pullProjectSource(ctx, project, workDir, branch, displayURL, shellEnv, log, &o)
		if err == nil {
			return nil
		}
		if !git.IsRecoverableSyncError(err) {
			return err
		}
		if !o.compact {
			log.step("Source sync failed on incomplete checkout; removing and re-cloning")
		}
		_ = os.RemoveAll(workDir)
	}

	cloneErr := r.cloneProjectSource(ctx, project, workDir, branch, displayURL, shellEnv, cloneURL, log, &o)
	if cloneErr != nil {
		_ = os.RemoveAll(workDir)
	}
	return cloneErr
}

func (r *Runner) pullProjectSource(
	ctx context.Context,
	project *repository.PackProject,
	workDir, branch, displayURL string,
	shellEnv []string,
	log *buildLogger,
	o *syncSourceOpts,
) error {
	if o.compact {
		log.step("  %s -> %s", project.ProjectName, branch)
	} else {
		if _, err := log.runGit(ctx, workDir, shellEnv, "rev-parse", "--is-inside-work-tree"); err != nil {
			return err
		}
		if _, err := log.runGit(ctx, workDir, shellEnv, "config", "--get", "remote.origin.url"); err != nil {
			log.step("remote.origin.url not set; continuing with configured URL: %s", displayURL)
		}
		log.step("Fetching upstream changes from %s", displayURL)
	}
	fetchArgs := git.FetchOriginArgs("origin", branch)
	if !o.compact {
		if _, err := log.runGit(ctx, workDir, shellEnv, fetchArgs...); err != nil {
			return fmt.Errorf("git fetch %s: %w", project.ProjectName, err)
		}
	} else if err := git.FetchOrigin(ctx, workDir, shellEnv, "origin", branch); err != nil {
		return fmt.Errorf("git fetch %s: %w", project.ProjectName, err)
	}
	if err := checkoutProjectRef(ctx, workDir, shellEnv, branch, log); err != nil {
		return fmt.Errorf("checkout %s: %w", project.ProjectName, err)
	}
	if !o.compact {
		log.logGitCheckoutSummary(ctx, workDir, shellEnv, branch)
	}
	return nil
}

func (r *Runner) cloneProjectSource(
	ctx context.Context,
	project *repository.PackProject,
	workDir, branch, displayURL string,
	shellEnv []string,
	cloneURL string,
	log *buildLogger,
	o *syncSourceOpts,
) error {
	if o.compact {
		log.step("  %s -> %s (clone)", project.ProjectName, branch)
	} else {
		log.step("Cloning the remote Git repository")
		log.step("Cloning repository %s", displayURL)
		if git.IsTagRef(branch) {
			log.step("Checking out git tag %s", git.NormalizeTagName(branch))
		}
	}
	parent := filepath.Dir(workDir)
	cloneBranch := git.CloneBranchForSync(branch)
	args := []string{"clone", "--progress"}
	if cloneBranch != "" {
		args = append(args, "--branch", cloneBranch, "--single-branch")
	}
	logURL := sanitizeURLForLog(cloneURL)
	args = append(args, logURL, workDir)
	if !o.compact {
		log.cmd("git", args)
	}
	runArgs := []string{"clone", "--progress"}
	if cloneBranch != "" {
		runArgs = append(runArgs, "--branch", cloneBranch, "--single-branch")
	}
	runArgs = append(runArgs, cloneURL, workDir)
	res, err := shell.RunArgs(ctx, "git", runArgs, shell.Options{Dir: parent, Env: shellEnv, AllowNonZeroExit: true})
	if o.compact {
		if err != nil {
			return fmt.Errorf("git clone %s: %w", project.ProjectName, err)
		}
		if res.ExitCode != 0 {
			if git.IsTagRef(branch) {
				return r.cloneThenCheckoutTag(ctx, project, workDir, branch, shellEnv, cloneURL, log, o)
			}
			return fmt.Errorf("git clone %s: exit %d", project.ProjectName, res.ExitCode)
		}
		return nil
	}
	log.emitOutput(res)
	if err != nil {
		return fmt.Errorf("git clone: %w", err)
	}
	if res.ExitCode != 0 {
		if git.IsTagRef(branch) {
			return r.cloneThenCheckoutTag(ctx, project, workDir, branch, shellEnv, cloneURL, log, o)
		}
		return fmt.Errorf("git clone: exit %d", res.ExitCode)
	}
	log.logGitCheckoutSummary(ctx, workDir, shellEnv, branch)
	return nil
}

func (r *Runner) cloneThenCheckoutTag(
	ctx context.Context,
	project *repository.PackProject,
	workDir, branch string,
	shellEnv []string,
	cloneURL string,
	log *buildLogger,
	o *syncSourceOpts,
) error {
	parent := filepath.Dir(workDir)
	_ = os.RemoveAll(workDir)
	if !o.compact {
		log.step("Tag clone failed; cloning default branch then fetching tag %s", git.NormalizeTagName(branch))
	}
	res, err := shell.RunArgs(ctx, "git", []string{"clone", "--progress", cloneURL, workDir}, shell.Options{
		Dir: parent, Env: shellEnv, AllowNonZeroExit: true,
	})
	if !o.compact {
		log.emitOutput(res)
	}
	if err != nil {
		return fmt.Errorf("git clone %s: %w", project.ProjectName, err)
	}
	if res.ExitCode != 0 {
		return fmt.Errorf("git clone %s: exit %d", project.ProjectName, res.ExitCode)
	}
	if err := git.FetchOrigin(ctx, workDir, shellEnv, "origin", branch); err != nil {
		return fmt.Errorf("git fetch tag %s: %w", project.ProjectName, err)
	}
	if err := checkoutProjectRef(ctx, workDir, shellEnv, branch, log); err != nil {
		return fmt.Errorf("checkout tag %s: %w", project.ProjectName, err)
	}
	if !o.compact {
		log.logGitCheckoutSummary(ctx, workDir, shellEnv, branch)
	}
	return nil
}

// syncSourceOpts controls logging verbosity for git sync.
type syncSourceOpts struct {
	compact bool
}

func checkoutProjectRef(ctx context.Context, workDir string, shellEnv []string, repoBranch string, log *buildLogger) error {
	repoBranch = strings.TrimSpace(repoBranch)
	var lastRes shell.Result
	var lastErr error
	recordFailure := func(res shell.Result, err error) {
		lastRes = res
		lastErr = err
	}
	if repoBranch != "" && !git.IsTagRef(repoBranch) {
		originRef := git.OriginBranchRef(repoBranch)
		if git.RefExists(ctx, workDir, shellEnv, originRef) {
			res, err := log.runGit(ctx, workDir, shellEnv, "checkout", "-f", "-B", repoBranch, originRef)
			if err == nil {
				return nil
			}
			recordFailure(res, err)
		}
	}
	for _, ref := range git.CheckoutRefCandidates(repoBranch) {
		res, err := log.runGit(ctx, workDir, shellEnv, "checkout", "-f", ref)
		if err == nil {
			return nil
		}
		recordFailure(res, err)
	}
	if git.IsTagRef(repoBranch) {
		return fmt.Errorf("git checkout tag ref %s: %s", repoBranch, describeCheckoutFailure(lastRes, lastErr, true))
	}
	res, err := log.runGit(ctx, workDir, shellEnv, "checkout", "-f", "origin/"+repoBranch)
	if err == nil {
		return nil
	}
	recordFailure(res, err)
	return fmt.Errorf("git checkout %s: %s", repoBranch, describeCheckoutFailure(lastRes, lastErr, false))
}

func describeCheckoutFailure(res shell.Result, err error, isTag bool) string {
	stderr := strings.TrimSpace(res.Stderr)
	if strings.Contains(stderr, "index.lock") {
		return "repository index locked by another git process or parallel build"
	}
	if stderr != "" {
		return gitFirstLine(stderr)
	}
	if isTag {
		return "tag not found locally (run fetch --tags)"
	}
	if git.IsCheckoutPathspecError(stderr) {
		return gitFirstLine(stderr) + " (local repository may be incomplete from interrupted sync)"
	}
	if err != nil {
		return err.Error()
	}
	return "branch or tag not found"
}

func gitFirstLine(s string) string {
	if i := strings.IndexAny(s, "\r\n"); i >= 0 {
		return strings.TrimSpace(s[:i])
	}
	return strings.TrimSpace(s)
}

func (r *Runner) gitShellEnv(
	project *repository.PackProject,
	repoURL string,
) (envtypes.GitConfig, []string, string, string, error) {
	shellEnv := append([]string{}, os.Environ()...)
	cloneURL := repoURL
	authLabel := ""
	var cfg envtypes.GitConfig
	if project.GitEnvRecID == "" {
		return cfg, shellEnv, cloneURL, authLabel, nil
	}
	item, err := r.env.FindByRecID(project.GitEnvRecID)
	if err != nil {
		return cfg, nil, "", "", fmt.Errorf("git environment: %w", err)
	}
	authLabel = item.EnvName
	if err := json.Unmarshal([]byte(item.ConfigJSON), &cfg); err != nil {
		return cfg, nil, "", "", fmt.Errorf("git environment config: %w", err)
	}
	auth := strings.ToLower(strings.TrimSpace(cfg.AuthType))
	if auth == envtypes.GitAuthHTTPS {
		cloneURL = embedHTTPSAuth(repoURL, cfg)
	} else if auth == envtypes.GitAuthSSH && strings.TrimSpace(cfg.PrivateKeyPath) != "" {
		keyPath := strings.TrimSpace(cfg.PrivateKeyPath)
		sshCmd := fmt.Sprintf("ssh -i %q -o StrictHostKeyChecking=accept-new", keyPath)
		if pass := strings.TrimSpace(cfg.PrivateKeyPassphrase); pass != "" {
			sshCmd = fmt.Sprintf("SSH_ASKPASS_REQUIRE=force ssh -i %q -o StrictHostKeyChecking=accept-new", keyPath)
		}
		shellEnv = append(shellEnv, "GIT_SSH_COMMAND="+sshCmd)
	}
	return cfg, shellEnv, cloneURL, authLabel, nil
}

func ensureProjectParentDir(workDir string) error {
	parent := filepath.Dir(workDir)
	if parent == "" || parent == "." {
		return nil
	}
	return os.MkdirAll(parent, 0o755)
}
