package project

import (
	"path/filepath"
	"sort"
	"strings"
)

// SourceContext 供 local_source_path 等配置项展开 AP_* 占位符（浏览目录、构建前解析，不含作业中才有字段时可留空）。
type SourceContext struct {
	WorkspaceID   string
	WorkspaceCode string
	WorkspaceName string
	WorkspacePath string
	ProjectID     string
	ProjectName   string
	ProjectType   string
	RepoBranch    string
	RepoURL       string
	JobID         string
	JobType       string
	Tag           string
	ProjectPath   string
	ProjectWorkdir string
}

// SourceContextFrom 从工程与空间字段构造展开上下文。
func SourceContextFrom(p Project, workspaceID, workspaceCode, workspaceName, workspacePath string) SourceContext {
	return SourceContext{
		WorkspaceID:   strings.TrimSpace(workspaceID),
		WorkspaceCode: strings.TrimSpace(workspaceCode),
		WorkspaceName: strings.TrimSpace(workspaceName),
		WorkspacePath: strings.TrimSpace(workspacePath),
		ProjectID:     strings.TrimSpace(p.RecID),
		ProjectName:   strings.TrimSpace(p.ProjectName),
		ProjectType:   strings.TrimSpace(p.ProjectType),
		RepoBranch:    strings.TrimSpace(p.RepoBranch),
		RepoURL:       strings.TrimSpace(p.RepoURL),
	}
}

// ResolveLocalSourcePath 展开 local_source_path 中的 AP_* 占位符。
func ResolveLocalSourcePath(r Recipe, ctx SourceContext) string {
	if !r.IsLocal() {
		return ""
	}
	return ExpandSourceVars(strings.TrimSpace(r.LocalSourcePath), ctx, r)
}

// AbsLocalSourceCtx 在展开占位符后取本机绝对源码目录。
func AbsLocalSourceCtx(r Recipe, ctx SourceContext) string {
	p := ResolveLocalSourcePath(r, ctx)
	if !IsAbsLocalPath(p) {
		return ""
	}
	return filepath.Clean(filepath.FromSlash(p))
}

// WorkRelCtx 与 WorkRel 相同，但会先展开 local_source_path 中的变量。
func WorkRelCtx(name string, r Recipe, ctx SourceContext) string {
	name = strings.TrimSpace(name)
	if AbsLocalSourceCtx(r, ctx) != "" {
		return name
	}
	extra := filepath.ToSlash(ResolveLocalSourcePath(r, ctx))
	extra = strings.Trim(extra, "/")
	if r.IsLocal() && extra != "" {
		if name == "" {
			return extra
		}
		return name + "/" + extra
	}
	return name
}

// ExpandSourceVars 将 s 中的 $VAR / ${VAR} / %VAR% 替换为 ctx 与 recipe 推导出的 AP_* 值。
func ExpandSourceVars(s string, ctx SourceContext, r Recipe) string {
	if s == "" {
		return s
	}
	vars := sourceVarMap(ctx, r)
	if len(vars) == 0 {
		return s
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
	out := s
	for _, key := range keys {
		val := vars[key]
		out = strings.ReplaceAll(out, "${"+key+"}", val)
		out = strings.ReplaceAll(out, "%"+key+"%", val)
	}
	for _, key := range keys {
		out = replaceDollarSourceVar(out, key, vars[key])
	}
	return out
}

func sourceVarMap(ctx SourceContext, r Recipe) map[string]string {
	vars := map[string]string{
		"AP_WORKSPACE_ID":   ctx.WorkspaceID,
		"AP_WORKSPACE_CODE": ctx.WorkspaceCode,
		"AP_WORKSPACE_NAME": ctx.WorkspaceName,
		"AP_WORKSPACE_PATH": ctx.WorkspacePath,
		"AP_PROJECT_ID":     ctx.ProjectID,
		"AP_PROJECT_NAME":   ctx.ProjectName,
		"AP_PROJECT_TYPE":   ctx.ProjectType,
		"AP_REPO_BRANCH":    ctx.RepoBranch,
		"AP_REPO_URL":       ctx.RepoURL,
		"AP_JOB_ID":         ctx.JobID,
		"AP_JOB_TYPE":       ctx.JobType,
		"AP_TAG":            ctx.Tag,
		"AP_PROJECT_PATH":   ctx.ProjectPath,
		"AP_PROJECT_WORKDIR": ctx.ProjectWorkdir,
	}
	if r.IsLocal() {
		vars["AP_SCM_MODE"] = "local"
	} else {
		vars["AP_SCM_MODE"] = "git"
	}
	if r.IsLocal() {
		raw := strings.TrimSpace(r.LocalSourcePath)
		if raw != "" && !containsSourceVar(raw) {
			vars["AP_LOCAL_SOURCE_PATH"] = filepath.ToSlash(raw)
		}
	}
	return vars
}

func containsSourceVar(s string) bool {
	return strings.Contains(s, "${AP_") || strings.Contains(s, "$AP_") || strings.Contains(s, "%AP_")
}

func replaceDollarSourceVar(s, key, val string) string {
	prefix := "$" + key
	if !strings.Contains(s, prefix) {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); {
		if s[i] == '$' && strings.HasPrefix(s[i:], prefix) {
			next := i + len(prefix)
			if next >= len(s) || !isSourceIdentByte(s[next]) {
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

func isSourceIdentByte(c byte) bool {
	return (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '_'
}
