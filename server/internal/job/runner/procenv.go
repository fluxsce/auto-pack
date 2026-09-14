package runner

import (
	"context"
	"path/filepath"
	"strings"

	"auto-pack/server/internal/env"
	"auto-pack/server/internal/job"
	"auto-pack/server/internal/project"
	"auto-pack/server/internal/workspace"
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
	wsCode := e.workspaceCode(ctx, j.WorkspaceRecID)
	wsName := e.workspaceTitle(ctx, j.WorkspaceRecID)
	wsPath := ""
	if wsCode != "" && j.WorkspaceRecID != "" {
		wsPath = workspace.Dir(e.wsRoot, wsCode, j.WorkspaceRecID)
	}
	extra = append(extra, jobContextEnv(j, prj, p, wsCode, wsName, wsPath, work, dir)...)
	return extra
}

// jobContextEnv 返回作业/空间/工程固有变量，供构建命令与构建后脚本引用。
func jobContextEnv(j job.Job, prj project.Project, p job.Payload, wsCode, wsName, wsPath, work, dir string) []string {
	branch := strings.TrimSpace(p.Branch)
	if branch == "" {
		branch = strings.TrimSpace(prj.RepoBranch)
	}
	repo := strings.TrimSpace(p.CloneURL)
	if repo == "" {
		repo = strings.TrimSpace(prj.RepoURL)
	}
	projectID := strings.TrimSpace(j.ProjectRecID)
	if projectID == "" {
		projectID = strings.TrimSpace(prj.RecID)
	}
	work = filepath.Clean(work)
	dir = filepath.Clean(dir)
	if dir == "" {
		dir = work
	}
	return []string{
		envKV("AP_JOB_ID", j.RecID),
		envKV("AP_JOB_TYPE", j.JobType),
		envKV("AP_WORKSPACE_ID", j.WorkspaceRecID),
		envKV("AP_WORKSPACE_CODE", wsCode),
		envKV("AP_WORKSPACE_NAME", wsName),
		envKV("AP_WORKSPACE_PATH", wsPath),
		envKV("AP_PROJECT_ID", projectID),
		envKV("AP_PROJECT_NAME", prj.ProjectName),
		envKV("AP_PROJECT_TYPE", prj.ProjectType),
		envKV("AP_PROJECT_PATH", work),
		envKV("AP_PROJECT_WORKDIR", dir),
		envKV("AP_REPO_BRANCH", branch),
		envKV("AP_REPO_URL", repo),
		envKV("AP_TAG", p.Tag),
	}
}

func envKV(key, val string) string {
	return key + "=" + strings.TrimSpace(val)
}
