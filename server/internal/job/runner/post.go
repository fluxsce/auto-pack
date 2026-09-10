package runner

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"auto-pack/server/internal/env"
	"auto-pack/server/internal/host/bash"
	"auto-pack/server/internal/host/ftp"
	hostssh "auto-pack/server/internal/host/ssh"
	"auto-pack/server/internal/job"
	"auto-pack/server/internal/project"
)

func (e *Engine) stepPost(ctx context.Context, j job.Job, prj project.Project, p job.Payload, work string, w io.Writer) error {
	recipe := project.ParseRecipe(prj.RecipeJSON)
	actions := recipe.Actions()
	if len(actions) == 0 {
		logLine(w, "无构建后操作")
		return nil
	}
	for i, act := range actions {
		n := i + 1
		title := postActionTitle(act.Kind)
		when := postWhenTitle(project.ActionWhen(act))
		if !project.ActionEnabled(act) {
			logLine(w, "%d. %s：跳过（已停用）", n, title)
			continue
		}
		if project.IsEmailAction(act.Kind) {
			if skipBatchSideEffects(j) {
				logLine(w, "%d. %s：跳过（工作空间批次不按工程发信）", n, title)
				continue
			}
			logLine(w, "%d. %s：作业结束后发送（%s）", n, title, when)
			continue
		}
		if !project.ActionMatches(act, true) {
			logLine(w, "%d. %s：跳过（%s）", n, title, when)
			continue
		}
		logLine(w, "%d. %s：开始（%s）", n, title, when)
		if err := e.runPostAction(ctx, j, prj, p, recipe, act, work, w); err != nil {
			logLine(w, "%d. %s：失败 %v", n, title, err)
			return fmt.Errorf("%s: %w", act.Kind, err)
		}
		logLine(w, "%d. %s：完成", n, title)
	}
	return nil
}

func postActionTitle(kind string) string {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "ftp":
		return "FTP 上传"
	case "sftp":
		return "SFTP 上传"
	case "ssh":
		return "SSH 上传"
	case "email", "smtp":
		return "邮件通知"
	case "shell":
		return "执行脚本"
	default:
		return kind
	}
}

func postWhenTitle(when string) string {
	switch when {
	case project.WhenFailure:
		return "仅失败"
	case project.WhenAlways:
		return "成功或失败"
	default:
		return "仅成功"
	}
}

func (e *Engine) runPostAction(ctx context.Context, j job.Job, prj project.Project, p job.Payload, recipe project.Recipe, act project.PostAction, work string, w io.Writer) error {
	kind := strings.ToLower(strings.TrimSpace(act.Kind))
	switch kind {
	case "ftp", "sftp", "ssh":
		envID := strings.TrimSpace(act.EnvRecID)
		if envID == "" {
			envID = strings.TrimSpace(p.EnvRecID)
		}
		got, cfg, err := e.uploadEnv(ctx, j.WorkspaceRecID, envID)
		if err != nil {
			return err
		}
		remote := strings.TrimSpace(act.RemotePath)
		if remote == "" {
			remote = strings.TrimSpace(p.RemotePath)
		}
		return e.putRemote(got, cfg, remote, work, p, recipe, w)
	case "email", "smtp":
		return e.sendActionMail(ctx, j, prj, act, true, "", w)
	case "shell":
		cmd := strings.TrimSpace(act.Command)
		if cmd == "" {
			return fmt.Errorf("shell command is empty")
		}
		dir := work
		if recipe.Workdir != "" {
			dir = filepath.Join(work, recipe.Workdir)
		}
		var extra []string
		if e.settings != nil {
			extra = e.settings.ProcEnv(ctx)
		}
		return bash.Run(ctx, dir, cmd, extra, w)
	default:
		return fmt.Errorf("unknown post action")
	}
}

func (e *Engine) putRemote(kind string, cfg map[string]any, remote, work string, p job.Payload, recipe project.Recipe, w io.Writer) error {
	return e.uploadArtifacts(work, p, recipe, w, func(local, name string) error {
		switch kind {
		case "sftp", "ssh":
			sc := hostssh.MapConfig(cfg)
			if remote != "" {
				sc.RemotePath = remote
			}
			return hostssh.UploadFile(sc, local, name, w)
		default:
			fc := ftp.MapConfig(cfg)
			if remote != "" {
				fc.RemoteDir = remote
			}
			return ftp.UploadFile(fc, local, name, w)
		}
	})
}

func (e *Engine) uploadEnv(ctx context.Context, workspaceID, recID string) (string, map[string]any, error) {
	if e.envs == nil {
		return "", nil, fmt.Errorf("env service not configured")
	}
	if recID != "" {
		row, cfg, err := e.envs.DecryptRow(ctx, recID)
		if err != nil {
			return "", nil, err
		}
		kind, err := env.NormalizeKind(row.EnvKind)
		if err != nil {
			return "", nil, err
		}
		return kind, cfg, nil
	}
	if _, cfg, err := e.envs.FindKind(ctx, workspaceID, "ftp"); err == nil {
		return "ftp", cfg, nil
	}
	if _, cfg, err := e.envs.FindKind(ctx, workspaceID, "sftp"); err == nil {
		return "sftp", cfg, nil
	}
	if _, cfg, err := e.envs.FindKind(ctx, workspaceID, "ssh"); err == nil {
		return "ssh", cfg, nil
	}
	return "", nil, fmt.Errorf("ftp/sftp env not found")
}

func (e *Engine) uploadArtifacts(work string, p job.Payload, recipe project.Recipe, w io.Writer, put func(local, name string) error) error {
	var files []string
	var err error
	if len(p.Artifacts) > 0 {
		var cleanup func()
		files, cleanup, err = project.CollectPushPaths(work, p.Artifacts)
		if cleanup != nil {
			defer cleanup()
		}
	} else {
		files, err = project.CollectArtifacts(work, recipe, p.Artifact)
	}
	if err != nil {
		return err
	}
	for _, local := range files {
		name := filepath.Base(local)
		if strings.Contains(filepath.ToSlash(local), "ap-ftp-push-") && strings.HasSuffix(strings.ToLower(name), ".zip") {
			logLine(w, "目录已压缩为 %s", name)
		}
		if err := put(local, name); err != nil {
			return err
		}
	}
	return nil
}
