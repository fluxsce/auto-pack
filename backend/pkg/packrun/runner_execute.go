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
	"auto-pack/backend/pkg/projectcfg"
	"auto-pack/backend/pkg/projectfs"
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

	workDir := projectfs.ResolveProjectAbsDir(project, ws, r.dataDir)
	log.header(project.ProjectName, build.BuildNum, workDir)
	log.line("Trigger: %s", mode.String())
	log.line("Build started at %s", logTimestamp())

	status := repository.BuildStatusSuccess
	message := "build finished successfully"
	finished := time.Now().UTC()

	if err := r.syncSource(ctx, project, workDir, log); err != nil {
		status = repository.BuildStatusFailed
		message = err.Error()
		log.finished(status, message)
		_ = r.builds.UpdateStatus(buildRecID, status, message, nil, &finished)
		return
	}

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
		if err := os.MkdirAll(workDir, 0o755); err != nil {
			status = repository.BuildStatusFailed
			message = fmt.Sprintf("mkdir work dir: %v", err)
			log.finished(status, message)
			_ = r.builds.UpdateStatus(buildRecID, status, message, nil, &finished)
			return
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
	if err := r.runPostBuildActions(ctx, project, cfg, workDir, buildSucceeded, log); err != nil {
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
		case envtypes.TypeNodeJS:
			var cfg envtypes.NodeJSConfig
			if err := json.Unmarshal([]byte(item.ConfigJSON), &cfg); err != nil {
				return err
			}
			if p := strings.TrimSpace(cfg.NodePath); p != "" {
				log.step("Node.js: %s", p)
			}
		}
	}
	return nil
}

func (r *Runner) syncSource(ctx context.Context, project *repository.PackProject, workDir string, log *buildLogger) error {
	repoURL := strings.TrimSpace(project.RepoURL)
	if repoURL == "" {
		log.section("Source")
		log.step("No Git repository URL configured; using local workspace only.")
		return nil
	}

	log.section("Source")
	log.step("Fetching changes from the remote Git repository")

	gitEnv, shellEnv, cloneURL, authLabel, err := r.gitShellEnv(project, repoURL)
	if err != nil {
		return err
	}
	_ = gitEnv
	if authLabel != "" {
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

	if _, err := log.runGit(ctx, "", shellEnv, "--version"); err != nil {
		return err
	}

	if git.IsRepository(ctx, workDir) {
		if _, err := log.runGit(ctx, workDir, shellEnv, "rev-parse", "--is-inside-work-tree"); err != nil {
			return err
		}
		if _, err := log.runGit(ctx, workDir, shellEnv, "config", "--get", "remote.origin.url"); err != nil {
			log.step("remote.origin.url not set; continuing with configured URL: %s", displayURL)
		}
		log.step("Fetching upstream changes from %s", displayURL)
		if _, err := log.runGit(ctx, workDir, shellEnv, "fetch", "--tags", "--progress", "origin"); err != nil {
			return err
		}
		if _, err := log.runGit(ctx, workDir, shellEnv, "checkout", "-f", branch); err != nil {
			if _, err := log.runGit(ctx, workDir, shellEnv, "checkout", "-f", "origin/"+branch); err != nil {
				return fmt.Errorf("git checkout %s: %w", branch, err)
			}
		}
		log.logGitCheckoutSummary(ctx, workDir, shellEnv, branch)
		return nil
	}

	log.step("Cloning the remote Git repository")
	log.step("Cloning repository %s", displayURL)
	parent := filepath.Dir(workDir)
	args := []string{"clone", "--progress"}
	if branch != "" {
		args = append(args, "--branch", branch, "--single-branch")
	}
	logURL := sanitizeURLForLog(cloneURL)
	args = append(args, logURL, workDir)
	log.cmd("git", args)
	runArgs := []string{"clone", "--progress"}
	if branch != "" {
		runArgs = append(runArgs, "--branch", branch, "--single-branch")
	}
	runArgs = append(runArgs, cloneURL, workDir)
	res, err := shell.RunArgs(ctx, "git", runArgs, shell.Options{Dir: parent, Env: shellEnv, AllowNonZeroExit: true})
	log.emitOutput(res)
	if err != nil {
		return fmt.Errorf("git clone: %w", err)
	}
	if res.ExitCode != 0 {
		return fmt.Errorf("git clone: exit %d", res.ExitCode)
	}
	log.logGitCheckoutSummary(ctx, workDir, shellEnv, branch)
	return nil
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
