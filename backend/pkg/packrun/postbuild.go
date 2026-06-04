package packrun

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"auto-pack/backend/internal/repository"
	"auto-pack/backend/pkg/email"
	"auto-pack/backend/pkg/envtypes"
	"auto-pack/backend/pkg/ftpclient"
	"auto-pack/backend/pkg/projectcfg"
	"auto-pack/backend/pkg/sftpclient"
	"auto-pack/backend/pkg/sshclient"
)

func (r *Runner) runPostBuildActions(
	ctx context.Context,
	project *repository.PackProject,
	cfg projectcfg.Config,
	workDir string,
	buildSucceeded bool,
	log *buildLogger,
) error {
	actions := projectcfg.EffectivePostBuildActions(cfg.PostBuildActions, project.DeployEnvRecID)
	if len(actions) == 0 {
		return nil
	}

	log.section("Post-build")
	artifactBase := projectcfg.ArtifactBaseDir(workDir, cfg.OutputDir)
	defaultPattern := strings.TrimSpace(cfg.ArtifactPattern)
	if defaultPattern == "" {
		defaultPattern = "**/*"
	}

	for i, action := range actions {
		if !action.ShouldRun(buildSucceeded) {
			continue
		}
		label := actionLabel(action, i+1)
		log.step("Running post-build action: %s", label)

		var err error
		switch strings.TrimSpace(action.Type) {
		case projectcfg.PostBuildShellExec:
			err = r.runLocalShell(ctx, project, action, workDir, log)
		case projectcfg.PostBuildSSHExec:
			err = r.runSSHExec(ctx, action, log)
		case projectcfg.PostBuildSSHTransfer:
			err = r.runSSHTransfer(ctx, action, artifactBase, defaultPattern, log)
		case projectcfg.PostBuildEmailNotify:
			err = r.runEmailNotify(ctx, project, action, buildSucceeded, log)
		default:
			err = fmt.Errorf("unsupported post-build type %q", action.Type)
		}
		if err != nil {
			return fmt.Errorf("%s: %w", label, err)
		}
		log.step("Post-build action completed: %s", label)
	}
	return nil
}

func actionLabel(a projectcfg.PostBuildAction, index int) string {
	if n := strings.TrimSpace(a.Name); n != "" {
		return n
	}
	return fmt.Sprintf("#%d %s", index, a.Type)
}

func (r *Runner) runLocalShell(
	ctx context.Context,
	project *repository.PackProject,
	action projectcfg.PostBuildAction,
	workDir string,
	log *buildLogger,
) error {
	cmd := strings.TrimSpace(action.Command)
	if cmd == "" {
		return fmt.Errorf("shell_exec: command is empty")
	}
	shellEnv, err := r.buildShellEnv(project)
	if err != nil {
		return err
	}
	log.step("Local shell in workspace: %s", workDir)
	res, err := log.runShell(ctx, workDir, shellEnv, cmd)
	if err != nil {
		return err
	}
	if res.ExitCode != 0 {
		return fmt.Errorf("command exited with code %d", res.ExitCode)
	}
	return nil
}

func (r *Runner) runSSHExec(ctx context.Context, action projectcfg.PostBuildAction, log *buildLogger) error {
	envID := strings.TrimSpace(action.EnvRecID)
	if envID == "" {
		return fmt.Errorf("ssh_exec: remote environment is required")
	}
	item, err := r.env.FindByRecID(envID)
	if err != nil {
		return err
	}
	if item.EnvType != envtypes.TypeSSH {
		return fmt.Errorf("ssh_exec: environment must be ssh, got %s", item.EnvType)
	}
	var cfg envtypes.SSHConfig
	if err := json.Unmarshal([]byte(item.ConfigJSON), &cfg); err != nil {
		return fmt.Errorf("ssh config: %w", err)
	}
	cmd := strings.TrimSpace(action.Command)
	if cmd == "" {
		return fmt.Errorf("ssh_exec: command is empty")
	}
	log.step("SSH exec on %s (%s)", cfg.Host, item.EnvName)
	res, err := sshclient.RunCommand(ctx, cfg, cmd)
	log.emitSSHOutput(res.Stdout, res.Stderr)
	if err != nil {
		return err
	}
	if res.ExitCode != 0 {
		return fmt.Errorf("remote command exit %d", res.ExitCode)
	}
	return nil
}

func (r *Runner) runSSHTransfer(
	ctx context.Context,
	action projectcfg.PostBuildAction,
	artifactBase, defaultPattern string,
	log *buildLogger,
) error {
	envID := strings.TrimSpace(action.EnvRecID)
	if envID == "" {
		return fmt.Errorf("ssh_transfer: remote environment is required")
	}
	item, err := r.env.FindByRecID(envID)
	if err != nil {
		return err
	}

	sets := action.TransferSets
	if len(sets) == 0 {
		sets = []projectcfg.PostBuildTransferSet{{SourceFiles: defaultPattern, RemoteDir: "/"}}
	}

	for si, set := range sets {
		pattern := strings.TrimSpace(set.SourceFiles)
		if pattern == "" {
			pattern = defaultPattern
		}
		files, err := projectcfg.MatchArtifactFiles(artifactBase, pattern)
		if err != nil {
			return fmt.Errorf("transfer set %d: match files: %w", si+1, err)
		}
		if len(files) == 0 {
			return fmt.Errorf("transfer set %d: no files matched %q under %s", si+1, pattern, artifactBase)
		}
		log.step("Transfer set %d: %d file(s) matched %q", si+1, len(files), pattern)

		switch item.EnvType {
		case envtypes.TypeSSH, envtypes.TypeSFTP:
			if err := r.transferSFTP(ctx, item, set, artifactBase, files, log); err != nil {
				return fmt.Errorf("transfer set %d: %w", si+1, err)
			}
		case envtypes.TypeFTP:
			if err := r.transferFTP(item, set, artifactBase, files, log); err != nil {
				return fmt.Errorf("transfer set %d: %w", si+1, err)
			}
		default:
			return fmt.Errorf("ssh_transfer: unsupported env type %s", item.EnvType)
		}

		if exec := strings.TrimSpace(set.ExecCommand); exec != "" {
			if item.EnvType != envtypes.TypeSSH {
				log.step("Skipping remote exec (requires SSH environment)")
				continue
			}
			var sshCfg envtypes.SSHConfig
			if item.EnvType == envtypes.TypeSFTP {
				var sftpCfg envtypes.SFTPConfig
				if err := json.Unmarshal([]byte(item.ConfigJSON), &sftpCfg); err != nil {
					return err
				}
				sshCfg = envtypes.SSHConfig{
					Host: sftpCfg.Host, Port: sftpCfg.Port, Username: sftpCfg.Username,
					Password: sftpCfg.Password, PrivateKeyPath: sftpCfg.PrivateKeyPath, TimeoutSec: sftpCfg.TimeoutSec,
				}
			} else {
				if err := json.Unmarshal([]byte(item.ConfigJSON), &sshCfg); err != nil {
					return err
				}
			}
			log.step("Remote exec after transfer")
			res, err := sshclient.RunCommand(ctx, sshCfg, exec)
			log.emitSSHOutput(res.Stdout, res.Stderr)
			if err != nil {
				return err
			}
			if res.ExitCode != 0 {
				return fmt.Errorf("remote exec exit %d", res.ExitCode)
			}
		}
	}
	return nil
}

func (r *Runner) transferSFTP(
	ctx context.Context,
	item *repository.EnvSetting,
	set projectcfg.PostBuildTransferSet,
	artifactBase string,
	files []string,
	log *buildLogger,
) error {
	var sftpCfg envtypes.SFTPConfig
	switch item.EnvType {
	case envtypes.TypeSSH:
		var sshCfg envtypes.SSHConfig
		if err := json.Unmarshal([]byte(item.ConfigJSON), &sshCfg); err != nil {
			return err
		}
		sftpCfg = envtypes.SFTPConfig{
			Host: sshCfg.Host, Port: sshCfg.Port, Username: sshCfg.Username,
			Password: sshCfg.Password, PrivateKeyPath: sshCfg.PrivateKeyPath, TimeoutSec: sshCfg.TimeoutSec,
		}
	case envtypes.TypeSFTP:
		if err := json.Unmarshal([]byte(item.ConfigJSON), &sftpCfg); err != nil {
			return err
		}
	}
	log.step("SFTP upload to %s (%s)", sftpCfg.Host, item.EnvName)

	var items []sftpclient.UploadItem
	remoteDir := strings.TrimSpace(set.RemoteDir)
	for _, local := range files {
		rel := projectcfg.RemoteRelPath(local, artifactBase, set.RemovePrefix)
		remote := ftpclient.LocalRelToRemote(remoteDir, rel)
		remote = strings.ReplaceAll(remote, "\\", "/")
		log.step("  %s -> %s", local, remote)
		items = append(items, sftpclient.UploadItem{LocalPath: local, RemotePath: remote})
	}
	return sftpclient.UploadFiles(ctx, sftpCfg, items)
}

func (r *Runner) transferFTP(
	item *repository.EnvSetting,
	set projectcfg.PostBuildTransferSet,
	artifactBase string,
	files []string,
	log *buildLogger,
) error {
	var ftpCfg envtypes.FTPConfig
	if err := json.Unmarshal([]byte(item.ConfigJSON), &ftpCfg); err != nil {
		return err
	}
	log.step("FTP upload to %s (%s)", ftpCfg.Host, item.EnvName)

	var items []ftpclient.UploadItem
	remoteDir := strings.TrimSpace(set.RemoteDir)
	for _, local := range files {
		rel := projectcfg.RemoteRelPath(local, artifactBase, set.RemovePrefix)
		remote := ftpclient.LocalRelToRemote(remoteDir, rel)
		log.step("  %s -> %s", local, remote)
		items = append(items, ftpclient.UploadItem{LocalPath: local, RemotePath: remote})
	}
	return ftpclient.UploadFiles(ftpCfg, items)
}

func (r *Runner) runEmailNotify(
	ctx context.Context,
	project *repository.PackProject,
	action projectcfg.PostBuildAction,
	buildSucceeded bool,
	log *buildLogger,
) error {
	recipients := splitRecipients(action.Recipients)
	if len(recipients) == 0 {
		return fmt.Errorf("email_notify: recipients required")
	}

	var smtpCfg email.Config
	envID := strings.TrimSpace(action.EnvRecID)
	if envID != "" {
		item, err := r.env.FindByRecID(envID)
		if err != nil {
			return err
		}
		if item.EnvType != envtypes.TypeSMTP {
			return fmt.Errorf("email_notify: environment must be smtp, got %s", item.EnvType)
		}
		var raw envtypes.SMTPConfig
		if err := json.Unmarshal([]byte(item.ConfigJSON), &raw); err != nil {
			return err
		}
		smtpCfg = email.Config{
			Host: raw.Host, Port: raw.Port, Username: raw.Username, Password: raw.Password,
			From: raw.From, FromName: raw.FromName, TLS: raw.TLS, StartTLS: raw.StartTLS,
			InsecureSkipVerify: raw.InsecureSkipVerify, TimeoutSec: raw.TimeoutSec,
		}
	} else {
		return fmt.Errorf("email_notify: smtp environment is required")
	}

	client, err := email.New(smtpCfg)
	if err != nil {
		return err
	}

	status := "SUCCESS"
	if !buildSucceeded {
		status = "FAILURE"
	}
	subject := strings.TrimSpace(action.Subject)
	if subject == "" {
		subject = fmt.Sprintf("[Auto Pack] %s %s #%s", project.ProjectName, status, project.ProjectVersion)
	} else {
		subject = expandNotifyTemplate(subject, project, status)
	}
	body := strings.TrimSpace(action.Body)
	if body == "" {
		body = fmt.Sprintf("Project: %s\nVersion: %s\nBuild status: %s\n", project.ProjectName, project.ProjectVersion, status)
	} else {
		body = expandNotifyTemplate(body, project, status)
	}

	log.step("Sending email to %s", strings.Join(recipients, ", "))
	msg := email.Message{To: recipients, Subject: subject, Text: body}
	return client.Send(ctx, msg)
}

func splitRecipients(raw string) []string {
	raw = strings.ReplaceAll(raw, ";", ",")
	parts := strings.Split(raw, ",")
	var out []string
	for _, p := range parts {
		if s := strings.TrimSpace(p); s != "" {
			out = append(out, s)
		}
	}
	return out
}

func expandNotifyTemplate(s string, project *repository.PackProject, status string) string {
	r := strings.NewReplacer(
		"{project}", project.ProjectName,
		"{version}", project.ProjectVersion,
		"{status}", status,
	)
	return r.Replace(s)
}

func (l *buildLogger) emitSSHOutput(stdout, stderr string) {
	if out := strings.TrimRight(stdout, "\r\n"); out != "" {
		l.line(out)
	}
	if errOut := strings.TrimRight(stderr, "\r\n"); errOut != "" {
		l.line(errOut)
	}
}
