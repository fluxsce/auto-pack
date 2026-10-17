package runner

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"auto-pack/server/internal/host/ftp"
	hostoss "auto-pack/server/internal/host/oss"
	hostssh "auto-pack/server/internal/host/ssh"
	"auto-pack/server/internal/host/utils"
	"auto-pack/server/internal/job"
	"auto-pack/server/internal/project"
)

type remoteStore interface {
	Exists(remote string) (bool, error)
	Download(remote string) ([]byte, error)
	UploadFile(local, remote string, w io.Writer) error
	UploadBytes(remote string, data []byte, w io.Writer) error
	Backup(src, dest string, w io.Writer) error
	Close() error
}

type notePair struct {
	backup string
	update string
}

func (e *Engine) stepFTP(ctx context.Context, j job.Job, prj project.Project, p job.Payload, work string, w io.Writer) error {
	recipe := project.ParseRecipe(prj.RecipeJSON)
	envID := strings.TrimSpace(p.EnvRecID)
	remote := strings.TrimSpace(p.RemotePath)
	if envID == "" {
		envID = recipe.FTPEnvRecID
	}
	if remote == "" {
		remote = strings.TrimSpace(recipe.RemotePath)
	}
	if envID == "" {
		for _, act := range recipe.Actions() {
			kind := strings.ToLower(strings.TrimSpace(act.Kind))
			if project.ActionEnabled(act) && (kind == "ftp" || kind == "sftp" || kind == "ssh") {
				envID = act.EnvRecID
				if remote == "" {
					remote = act.RemotePath
				}
				break
			}
		}
	}
	kind, cfg, err := e.uploadEnv(ctx, j.WorkspaceRecID, envID)
	if err != nil {
		return fmt.Errorf("upload env: %w", err)
	}
	store, err := openRemote(kind, cfg)
	if err != nil {
		return err
	}
	defer store.Close()
	extras, err := e.resolveMergeSources(ctx, j, p)
	if err != nil {
		return err
	}
	return e.pushWithBackup(store, remote, work, p, recipe, extras, w)
}

func (e *Engine) resolveMergeSources(ctx context.Context, j job.Job, p job.Payload) ([]project.PushSource, error) {
	if len(p.MergeSources) == 0 {
		return nil, nil
	}
	if e.projects == nil {
		return nil, fmt.Errorf("merge sources: project service required")
	}
	out := make([]project.PushSource, 0, len(p.MergeSources))
	seen := map[string]struct{}{}
	for _, src := range p.MergeSources {
		id := strings.TrimSpace(src.ProjectRecID)
		rel := strings.TrimSpace(src.Path)
		if id == "" || rel == "" {
			return nil, fmt.Errorf("merge source project and path required")
		}
		key := id + "\x00" + rel
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		prj, err := e.projects.GetRaw(ctx, id)
		if err != nil {
			return nil, fmt.Errorf("merge source %s: %w", id, err)
		}
		if prj.WorkspaceRecID != j.WorkspaceRecID {
			return nil, fmt.Errorf("merge source %s outside workspace", id)
		}
		out = append(out, project.PushSource{Work: e.workDir(ctx, j, prj, job.Payload{}), Rel: rel})
	}
	return out, nil
}

func openRemote(kind string, cfg map[string]any) (remoteStore, error) {
	switch kind {
	case "sftp", "ssh":
		return hostssh.DialSession(hostssh.MapConfig(cfg))
	case "oss":
		return hostoss.Open(hostoss.MapConfig(cfg))
	default:
		return ftp.Dial(ftp.MapConfig(cfg))
	}
}

func (e *Engine) pushWithBackup(store remoteStore, pushPath, work string, p job.Payload, recipe project.Recipe, extras []project.PushSource, w io.Writer) error {
	var files []string
	var err error
	if len(p.Artifacts) > 0 || len(extras) > 0 {
		var cleanup func()
		srcs := make([]project.PushSource, 0, len(p.Artifacts)+len(extras))
		for _, rel := range p.Artifacts {
			srcs = append(srcs, project.PushSource{Work: work, Rel: rel})
		}
		srcs = append(srcs, extras...)
		files, cleanup, err = project.CollectPushSources(srcs)
		if cleanup != nil {
			defer cleanup()
		}
	} else {
		files, err = project.CollectArtifacts(work, recipe, p.Artifact)
	}
	if err != nil {
		return err
	}
	pushTime := time.Now()
	pairs := make([]notePair, 0, len(files))
	for _, local := range files {
		name := filepath.Base(local)
		if strings.Contains(filepath.ToSlash(local), "ap-ftp-push-") && strings.HasSuffix(strings.ToLower(name), ".zip") {
			if project.IsSceMergeZipName(name) {
				logLine(w, "sce 前端目录已合并为 %s", name)
			} else {
				logLine(w, "目录已压缩为 %s", name)
			}
		}
		pair, err := pushOne(store, pushPath, local, name, p.BackupPath, pushTime, w)
		if err != nil {
			return err
		}
		pairs = append(pairs, pair)
	}
	return writeUpdateNote(store, pushPath, files, p, pairs, pushTime, w)
}

func pushOne(store remoteStore, pushPath, local, name, backupDir string, at time.Time, w io.Writer) (notePair, error) {
	pushDir, updatePath := utils.PushLayout(pushPath, name)
	if strings.TrimSpace(backupDir) == "" {
		backupDir = utils.DefaultBackupDir(pushDir, at)
	} else {
		backupDir = utils.RemoteDir(backupDir)
	}
	backupPath := utils.RemoteJoin(backupDir, name)
	exists, err := store.Exists(updatePath)
	if err != nil {
		return notePair{}, err
	}
	if exists {
		logLine(w, "远端已有 %s，先备份", updatePath)
		if err := store.Backup(updatePath, backupPath, w); err != nil {
			return notePair{}, fmt.Errorf("backup %s: %w", updatePath, err)
		}
		logLine(w, "备份完成 %s", backupPath)
	} else {
		logLine(w, "远端没有 %s，跳过备份", updatePath)
		backupPath = ""
	}
	logLine(w, "上传 %s", updatePath)
	if err := store.UploadFile(local, updatePath, w); err != nil {
		return notePair{}, err
	}
	return notePair{backup: backupPath, update: updatePath}, nil
}

func writeUpdateNote(store remoteStore, pushPath string, files []string, p job.Payload, pairs []notePair, at time.Time, w io.Writer) error {
	pusher := strings.TrimSpace(p.Pusher)
	taskRef := strings.TrimSpace(p.TaskRef)
	if taskRef == "" {
		taskRef = strings.TrimSpace(p.Notes)
	}
	if pusher == "" || taskRef == "" {
		logLine(w, "未填推送人或变更说明，跳过写入 %s", utils.UpdateNoteName)
		return nil
	}
	sample := ""
	if len(files) > 0 {
		sample = filepath.Base(files[0])
	}
	pushDir, _ := utils.PushLayout(pushPath, sample)
	infoPath := utils.RemoteJoin(pushDir, utils.UpdateNoteName)
	manifest := buildUpdateNote(pusher, taskRef, p.Detail, pairs, at)
	exists, err := store.Exists(infoPath)
	if err != nil {
		return err
	}
	note := manifest
	if exists {
		logLine(w, "远端已有 %s，追加本次记录", infoPath)
		existing, err := store.Download(infoPath)
		if err != nil {
			return fmt.Errorf("read %s: %w", infoPath, err)
		}
		note = mergeUpdateNote(string(existing), manifest)
	} else {
		logLine(w, "远端没有 %s，将新建", infoPath)
	}
	if err := store.UploadBytes(infoPath, []byte(note), w); err != nil {
		return fmt.Errorf("write %s: %w", infoPath, err)
	}
	logLine(w, "已写入 %s", infoPath)
	return nil
}

func buildUpdateNote(pusher, taskRef, detail string, pairs []notePair, at time.Time) string {
	if at.IsZero() {
		at = time.Now()
	}
	var backups, updates []string
	for _, p := range pairs {
		if s := strings.TrimSpace(p.backup); s != "" {
			backups = append(backups, s)
		}
		if s := strings.TrimSpace(p.update); s != "" {
			updates = append(updates, s)
		}
	}
	lines := []string{
		fmt.Sprintf("%s %s %s", at.Format("20060102"), strings.TrimSpace(pusher), strings.TrimSpace(taskRef)),
		"备份包路径:" + strings.Join(backups, ";"),
		"更新包路径:" + strings.Join(updates, ";"),
	}
	if extra := formatNoteDetail(detail); extra != "" {
		lines = append(lines, extra)
	}
	return strings.Join(lines, "\n") + "\n"
}

func formatNoteDetail(raw string) string {
	raw = strings.TrimRight(strings.ReplaceAll(raw, "\r\n", "\n"), "\n")
	if strings.TrimSpace(raw) == "" {
		return ""
	}
	var out []string
	for _, line := range strings.Split(raw, "\n") {
		trimmed := strings.TrimRight(line, " \t")
		if strings.TrimSpace(trimmed) == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "    ") || strings.HasPrefix(trimmed, "\t") {
			out = append(out, trimmed)
			continue
		}
		out = append(out, "    "+strings.TrimLeft(trimmed, " \t"))
	}
	return strings.Join(out, "\n")
}

func mergeUpdateNote(existing, manifest string) string {
	manifest = strings.TrimRight(strings.ReplaceAll(manifest, "\r\n", "\n"), "\n")
	existing = strings.TrimRight(strings.ReplaceAll(existing, "\r\n", "\n"), "\n")
	if manifest == "" {
		if existing == "" {
			return ""
		}
		return existing + "\n"
	}
	if existing == "" {
		return manifest + "\n"
	}
	return existing + "\n\n" + manifest + "\n"
}
