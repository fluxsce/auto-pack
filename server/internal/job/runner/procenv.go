package runner

import (
	"context"
	"path/filepath"
	"sort"
	"strings"

	"auto-pack/server/internal/env"
	"auto-pack/server/internal/job"
	"auto-pack/server/internal/project"
)

// procEnv 组装构建/后置脚本进程环境：工具链、全局变量、作业上下文。
func (e *Engine) procEnv(ctx context.Context, j job.Job, prj project.Project, p job.Payload, recipe project.Recipe, work, dir string) []string {
	extra := env.ProcEnv(
		e.envCfg(ctx, recipe.JDKEnvRecID, j.WorkspaceRecID, "jdk"),
		e.envCfg(ctx, recipe.MavenEnvRecID, j.WorkspaceRecID, "maven"),
		e.envCfg(ctx, recipe.NodeEnvRecID, j.WorkspaceRecID, "node"),
	)
	if e.settings != nil {
		extra = append(extra, e.settings.ProcEnv(ctx)...)
	}
	srcCtx := e.sourceContext(ctx, j, prj, p, work, dir)
	extra = append(extra, jobContextEnv(j, prj, recipe, srcCtx)...)
	return extra
}

// jobContextEnv 返回作业/空间/工程固有变量，供构建命令与构建后脚本引用。
func jobContextEnv(j job.Job, prj project.Project, recipe project.Recipe, ctx project.SourceContext) []string {
	projectID := strings.TrimSpace(j.ProjectRecID)
	if projectID == "" {
		projectID = strings.TrimSpace(prj.RecID)
	}
	scmMode := "git"
	if recipe.IsLocal() {
		scmMode = "local"
	}
	localSource := filepath.ToSlash(project.ResolveLocalSourcePath(recipe, ctx))
	return []string{
		envKV("AP_JOB_ID", ctx.JobID),
		envKV("AP_JOB_TYPE", ctx.JobType),
		envKV("AP_WORKSPACE_ID", ctx.WorkspaceID),
		envKV("AP_WORKSPACE_CODE", ctx.WorkspaceCode),
		envKV("AP_WORKSPACE_NAME", ctx.WorkspaceName),
		envKV("AP_WORKSPACE_PATH", ctx.WorkspacePath),
		envKV("AP_PROJECT_ID", projectID),
		envKV("AP_PROJECT_NAME", ctx.ProjectName),
		envKV("AP_PROJECT_TYPE", ctx.ProjectType),
		envKV("AP_PROJECT_PATH", ctx.ProjectPath),
		envKV("AP_PROJECT_WORKDIR", ctx.ProjectWorkdir),
		envKV("AP_SCM_MODE", scmMode),
		envKV("AP_LOCAL_SOURCE_PATH", localSource),
		envKV("AP_REPO_BRANCH", ctx.RepoBranch),
		envKV("AP_REPO_URL", ctx.RepoURL),
		envKV("AP_TAG", ctx.Tag),
	}
}

func envKV(key, val string) string {
	return key + "=" + strings.TrimSpace(val)
}

// expandProcEnvForLog 将命令中的 $VAR / ${VAR} / %VAR% 替换为 procEnv 值，仅用于日志展示。
func expandProcEnvForLog(cmd string, env []string) string {
	if cmd == "" || len(env) == 0 {
		return cmd
	}
	vars := procEnvMap(env)
	if len(vars) == 0 {
		return cmd
	}
	keys := make([]string, 0, len(vars))
	for key := range vars {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		if len(keys[i]) != len(keys[j]) {
			return len(keys[i]) > len(keys[j])
		}
		return keys[i] < keys[j]
	})
	out := cmd
	for _, key := range keys {
		val := vars[key]
		out = strings.ReplaceAll(out, "${"+key+"}", val)
		out = strings.ReplaceAll(out, "%"+key+"%", val)
	}
	for _, key := range keys {
		out = replaceDollarEnv(out, key, vars[key])
	}
	return out
}

func procEnvMap(env []string) map[string]string {
	vars := make(map[string]string, len(env))
	for _, pair := range env {
		i := strings.IndexByte(pair, '=')
		if i <= 0 {
			continue
		}
		vars[pair[:i]] = pair[i+1:]
	}
	return vars
}

func replaceDollarEnv(s, key, val string) string {
	prefix := "$" + key
	if !strings.Contains(s, prefix) {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); {
		if s[i] == '$' && strings.HasPrefix(s[i:], prefix) {
			next := i + len(prefix)
			if next >= len(s) || !isShellIdentByte(s[next]) {
				b.WriteString(val)
				i = next
				continue
			}
		}
		b.WriteByte(s[i])
		i++
	}
	return b.String()
}

func isShellIdentByte(c byte) bool {
	return (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '_'
}
