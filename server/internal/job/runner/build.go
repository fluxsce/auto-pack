package runner

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"auto-pack/server/internal/env"
	"auto-pack/server/internal/host/bash"
	"auto-pack/server/internal/host/git"
	"auto-pack/server/internal/host/maven"
	"auto-pack/server/internal/host/npm"
	"auto-pack/server/internal/job"
	"auto-pack/server/internal/project"
	"auto-pack/server/internal/release/pom"
)

func (e *Engine) stepBuild(ctx context.Context, j job.Job, prj project.Project, p job.Payload, work string, w io.Writer) error {
	recipe := project.ParseRecipe(prj.RecipeJSON)
	cmd := p.Command
	if cmd == "" {
		cmd = recipe.Command
	}
	dir := work
	if recipe.Workdir != "" {
		dir = filepath.Join(work, recipe.Workdir)
	}
	extra := e.procEnv(ctx, j, prj, p, recipe, work, dir)
	logToolchain(w, extra)
	if skipBuildAsSuccess(prj.ProjectType, cmd) {
		logLine(w, "custom 工程未配置构建命令，跳过构建步骤并记为成功")
		return nil
	}
	switch strings.ToLower(strings.TrimSpace(prj.ProjectType)) {
	case "npm", "node", "vue", "frontend":
		return npm.Run(ctx, dir, cmd, extra, w)
	case "maven", "java":
		mavenCfg := e.envCfg(ctx, recipe.MavenEnvRecID, j.WorkspaceRecID, "maven")
		cmd = env.MavenCommand(cmd, mapString(mavenCfg, "settings_xml"))
		return maven.Run(ctx, dir, cmd, extra, w)
	default:
		if cmd == "" {
			return fmt.Errorf("build command is empty")
		}
		return bash.Run(ctx, dir, cmd, extra, w)
	}
}

func logToolchain(w io.Writer, extra []string) {
	home := envValue(extra, "JAVA_HOME")
	if home == "" {
		logLine(w, "未从工程 JDK/Maven 环境注入 JAVA_HOME，将使用进程环境")
		return
	}
	logLine(w, "JAVA_HOME=%s", home)
}

func envValue(env []string, key string) string {
	prefix := key + "="
	for i := len(env) - 1; i >= 0; i-- {
		if strings.HasPrefix(env[i], prefix) {
			return env[i][len(prefix):]
		}
	}
	return ""
}

// skipBuildAsSuccess reports that a custom project with no command should skip compile and still succeed.
func skipBuildAsSuccess(projectType, command string) bool {
	if strings.TrimSpace(command) != "" {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(projectType)) {
	case "custom", "customer":
		return true
	default:
		return false
	}
}

func (e *Engine) stepVersionTxt(work string, p job.Payload, w io.Writer) error {
	if p.Tag == "" {
		return fmt.Errorf("tag is required")
	}
	path := filepath.Join(work, "src", "main", "resources", "Version.txt")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	block := fmt.Sprintf("Version: %s\n发布时间: %s\n版本更新概要:\n\t   · %s\n", p.Tag, job.NowUTC()[:10], strings.TrimSpace(p.Notes))
	existing, _ := os.ReadFile(path)
	out := block + "\n" + string(existing)
	_, _ = fmt.Fprintf(w, "write %s\n", path)
	return os.WriteFile(path, []byte(out), 0o644)
}

func (e *Engine) stepPOM(ctx context.Context, j job.Job, prj project.Project, p job.Payload, work string, w io.Writer) error {
	tag := strings.TrimSpace(p.POMVersion)
	if tag == "" {
		tag = strings.TrimSpace(p.Tag)
	}
	if tag == "" {
		return fmt.Errorf("pom version is required")
	}
	recipe := project.ParseRecipe(prj.RecipeJSON)
	root := work
	if extra := strings.TrimSpace(recipe.Workdir); extra != "" {
		root = filepath.Join(work, extra)
	}
	written := pom.PomVersionForTag(tag)
	logLine(w, "按 1.0 规则更新全部 pom.xml -> %s", written)
	files, steps, err := pom.CollectPomVersionUpdates(root, tag)
	if err != nil {
		return err
	}
	for _, msg := range steps {
		logLine(w, "%s", msg)
	}
	rels, err := writePomFiles(work, files)
	if err != nil {
		return err
	}
	if len(rels) == 0 {
		if j.JobType == job.TypePOMVersion {
			return fmt.Errorf("no pom.xml versions updated")
		}
		return nil
	}
	if j.JobType != job.TypePOMVersion {
		return nil
	}
	if recipe.IsLocal() {
		logLine(w, "scm_mode=local，只改本地文件，不提交")
		return nil
	}
	if !git.IsRepo(work) {
		return fmt.Errorf("not a git repo, cannot commit pom")
	}
	branch := strings.TrimSpace(p.Branch)
	if branch == "" {
		branch = strings.TrimSpace(prj.RepoBranch)
	}
	if branch != "" {
		logLine(w, "提交并推送到工程当前分支 %s（已按远程最新同步）", branch)
	}
	msg := fmt.Sprintf("chore: set pom version to %s", written)
	return git.CommitPush(ctx, work, msg, rels, e.gitOptFor(ctx, j.WorkspaceRecID, recipe.GitEnvRecID), w)
}

func writePomFiles(work string, files map[string][]byte) ([]string, error) {
	rels := make([]string, 0, len(files))
	for path, data := range files {
		if err := os.WriteFile(path, data, 0o644); err != nil {
			return nil, err
		}
		rel, err := filepath.Rel(work, path)
		if err != nil {
			rel = path
		}
		rels = append(rels, filepath.ToSlash(rel))
	}
	sort.Strings(rels)
	return rels, nil
}
