package projects

import (
	"context"
	"encoding/json"
	"errors"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"auto-pack/server/internal/audit"
	"auto-pack/server/internal/env"
	"auto-pack/server/internal/host/ftp"
	"auto-pack/server/internal/host/maven"
	hostoss "auto-pack/server/internal/host/oss"
	hostssh "auto-pack/server/internal/host/ssh"
	"auto-pack/server/internal/host/utils"
	"auto-pack/server/internal/project"
	"auto-pack/server/internal/settings"
	"auto-pack/server/internal/web/httpx"
	"auto-pack/server/internal/web/middleware"
	"auto-pack/server/internal/workspace"
)

// Controller 处理工程配方 HTTP。
type Controller struct {
	projects   *project.Service
	workspaces *workspace.Service
	envs       *env.Service
	settings   *settings.Service
	audit      *audit.Service
}

// New 构造工程控制器。
func New(projects *project.Service, workspaces *workspace.Service, envs *env.Service, settings *settings.Service, audit *audit.Service) *Controller {
	return &Controller{projects: projects, workspaces: workspaces, envs: envs, settings: settings, audit: audit}
}

// PreviewCron 用与保存相同的解析器预览接下来的触发时间。
func (c *Controller) PreviewCron(w http.ResponseWriter, r *http.Request) {
	expr := strings.TrimSpace(r.URL.Query().Get("expr"))
	if expr == "" && r.Body != nil {
		var in struct {
			Expr string `json:"expr"`
		}
		_ = json.NewDecoder(r.Body).Decode(&in)
		expr = strings.TrimSpace(in.Expr)
	}
	httpx.WriteJSON(w, http.StatusOK, project.PreviewCron(expr, 5))
}

// UpdateGitBranches 把空间内 Git 工程的检出分支改成同一值。
func (c *Controller) UpdateGitBranches(w http.ResponseWriter, r *http.Request) {
	var in struct {
		WorkspaceRecID string `json:"workspace_rec_id"`
		RepoBranch     string `json:"repo_branch"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid body")
		return
	}
	actor := middleware.ActorFrom(r)
	out, err := c.projects.UpdateGitBranches(r.Context(), actor, in.WorkspaceRecID, in.RepoBranch)
	c.audit.Note(r.Context(), actor, in.WorkspaceRecID, audit.ActionProjectBranch, "workspace", in.WorkspaceRecID, "git branch "+strings.TrimSpace(in.RepoBranch), err)
	if httpx.WriteSvcErr(w, err) {
		return
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}

// UpdateCommands 按类型批量改工程构建命令（token 替换或整条覆盖）。
func (c *Controller) UpdateCommands(w http.ResponseWriter, r *http.Request) {
	var in struct {
		WorkspaceRecID string               `json:"workspace_rec_id"`
		Patch          project.CommandPatch `json:"patch"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid body")
		return
	}
	actor := middleware.ActorFrom(r)
	out, err := c.projects.UpdateCommands(r.Context(), actor, in.WorkspaceRecID, in.Patch)
	c.audit.Note(r.Context(), actor, in.WorkspaceRecID, audit.ActionProjectCommand, "workspace", in.WorkspaceRecID, "batch command", err)
	if httpx.WriteSvcErr(w, err) {
		return
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}

// UpdatePostActions 把空间内每个工程的邮件或脚本构建后操作写成同一份。
func (c *Controller) UpdatePostActions(w http.ResponseWriter, r *http.Request) {
	var in struct {
		WorkspaceRecID string             `json:"workspace_rec_id"`
		Family         string             `json:"family,omitempty"`
		Action         project.PostAction `json:"action"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid body")
		return
	}
	actor := middleware.ActorFrom(r)
	out, err := c.projects.UpdatePostActions(r.Context(), actor, in.WorkspaceRecID, in.Family, in.Action)
	c.audit.Note(r.Context(), actor, in.WorkspaceRecID, audit.ActionProjectPost, "workspace", in.WorkspaceRecID, "post "+strings.TrimSpace(in.Action.Kind), err)
	if httpx.WriteSvcErr(w, err) {
		return
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}

// List 列出空间内工程。
func (c *Controller) List(w http.ResponseWriter, r *http.Request) {
	list, err := c.projects.List(r.Context(), middleware.ActorFrom(r), r.URL.Query().Get("workspace_rec_id"))
	if httpx.WriteSvcErr(w, err) {
		return
	}
	httpx.WriteJSON(w, http.StatusOK, list)
}

// Save 新建或更新工程。
func (c *Controller) Save(w http.ResponseWriter, r *http.Request) {
	var p project.Project
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid body")
		return
	}
	actor := middleware.ActorFrom(r)
	out, err := c.projects.Save(r.Context(), actor, p)
	c.audit.Note(r.Context(), actor, p.WorkspaceRecID, audit.ActionProjectSave, "project", firstNonEmpty(out.RecID, p.RecID), "save "+p.ProjectName, err)
	if httpx.WriteSvcErr(w, err) {
		return
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}

// Get 读取单个工程。
func (c *Controller) Get(w http.ResponseWriter, r *http.Request) {
	p, err := c.projects.Get(r.Context(), middleware.ActorFrom(r), r.PathValue("id"))
	if httpx.WriteSvcErr(w, err) {
		return
	}
	httpx.WriteJSON(w, http.StatusOK, p)
}

// Clone 复制工程配方到同一空间。
func (c *Controller) Clone(w http.ResponseWriter, r *http.Request) {
	actor := middleware.ActorFrom(r)
	id := r.PathValue("id")
	src, _ := c.projects.Get(r.Context(), actor, id)
	out, err := c.projects.Clone(r.Context(), actor, id)
	c.audit.Note(r.Context(), actor, src.WorkspaceRecID, audit.ActionProjectClone, "project", firstNonEmpty(out.RecID, id), "clone "+src.ProjectName, err)
	if httpx.WriteSvcErr(w, err) {
		return
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}

// Files 列举工程检出目录下一层文件。
func (c *Controller) Files(w http.ResponseWriter, r *http.Request) {
	p, err := c.projects.Get(r.Context(), middleware.ActorFrom(r), r.PathValue("id"))
	if httpx.WriteSvcErr(w, err) {
		return
	}
	tree, err := c.workspaces.ListCheckout(r.Context(), middleware.ActorFrom(r), p.WorkspaceRecID, c.checkoutWorkRel(r.Context(), p), r.URL.Query().Get("path"), c.checkoutAbs(r.Context(), p))
	if writePathErr(w, err) {
		return
	}
	httpx.WriteJSON(w, http.StatusOK, tree)
}

// ClearFiles 清空工程检出目录内的文件，保留目录本身。
func (c *Controller) ClearFiles(w http.ResponseWriter, r *http.Request) {
	actor := middleware.ActorFrom(r)
	p, err := c.projects.Get(r.Context(), actor, r.PathValue("id"))
	if httpx.WriteSvcErr(w, err) {
		return
	}
	err = c.workspaces.ClearCheckout(r.Context(), actor, p.WorkspaceRecID, c.checkoutWorkRel(r.Context(), p), c.checkoutAbs(r.Context(), p))
	c.audit.Note(r.Context(), actor, p.WorkspaceRecID, audit.ActionProjectClearFiles, "project", p.RecID, "clear checkout "+p.ProjectName, err)
	if writePathErr(w, err) {
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// Deps 用绑定的 JDK/Maven 跑 `mvn dependency:tree`；npm / Go 仍读清单文件。
func (c *Controller) Deps(w http.ResponseWriter, r *http.Request) {
	p, err := c.projects.Get(r.Context(), middleware.ActorFrom(r), r.PathValue("id"))
	if httpx.WriteSvcErr(w, err) {
		return
	}
	q := c.depsQuery(r.Context(), p)
	q.Refresh = r.URL.Query().Get("refresh") == "1"
	deps, err := c.workspaces.ListDeps(r.Context(), middleware.ActorFrom(r), p.WorkspaceRecID, c.checkoutWorkRel(r.Context(), p), c.checkoutAbs(r.Context(), p), q)
	if writePathErr(w, err) {
		return
	}
	httpx.WriteJSON(w, http.StatusOK, deps)
}

func (c *Controller) depsQuery(ctx context.Context, p project.Project) workspace.DepsQuery {
	recipe := project.ParseRecipe(p.RecipeJSON)
	mavenCfg := c.envCfg(ctx, recipe.MavenEnvRecID, p.WorkspaceRecID, "maven")
	extra := env.ProcEnv(
		c.envCfg(ctx, recipe.JDKEnvRecID, p.WorkspaceRecID, "jdk"),
		mavenCfg,
		c.envCfg(ctx, recipe.NodeEnvRecID, p.WorkspaceRecID, "node"),
	)
	if c.settings != nil {
		extra = append(extra, c.settings.ProcEnv(ctx)...)
	}
	return workspace.DepsQuery{
		ExtraEnv: extra,
		Command:  env.MavenCommand(maven.TreeCommand, mapString(mavenCfg, "settings_xml")),
		Workdir:  recipe.Workdir,
	}
}

func (c *Controller) envCfg(ctx context.Context, recID, workspaceID, kind string) map[string]any {
	if c.envs == nil {
		return nil
	}
	if strings.TrimSpace(recID) != "" {
		if _, cfg, err := c.envs.DecryptRow(ctx, recID); err == nil {
			return cfg
		}
	}
	if kind == "jdk" {
		return nil
	}
	return c.envs.KindConfig(ctx, workspaceID, kind)
}

func mapString(m map[string]any, key string) string {
	if m == nil {
		return ""
	}
	v, ok := m[key]
	if !ok || v == nil {
		return ""
	}
	s, _ := v.(string)
	return strings.TrimSpace(s)
}

// File 读取工程检出目录内的文本文件。
func (c *Controller) File(w http.ResponseWriter, r *http.Request) {
	p, err := c.projects.Get(r.Context(), middleware.ActorFrom(r), r.PathValue("id"))
	if httpx.WriteSvcErr(w, err) {
		return
	}
	blob, err := c.workspaces.ReadCheckout(r.Context(), middleware.ActorFrom(r), p.WorkspaceRecID, c.checkoutWorkRel(r.Context(), p), r.URL.Query().Get("path"), c.checkoutAbs(r.Context(), p))
	if writePathErr(w, err) {
		return
	}
	httpx.WriteJSON(w, http.StatusOK, blob)
}

const maxDownloadPaths = 400

// DownloadFile 下载工程检出目录内的文件或目录；多个 path 打成一个 zip（跳过嵌套 .git）。
func (c *Controller) DownloadFile(w http.ResponseWriter, r *http.Request) {
	p, err := c.projects.Get(r.Context(), middleware.ActorFrom(r), r.PathValue("id"))
	if httpx.WriteSvcErr(w, err) {
		return
	}
	rels := collectDownloadPaths(r.URL.Query())
	if len(rels) == 0 {
		httpx.WriteError(w, http.StatusBadRequest, "path required")
		return
	}
	actor := middleware.ActorFrom(r)
	workRel := c.checkoutWorkRel(r.Context(), p)
	absLocal := c.checkoutAbs(r.Context(), p)
	if len(rels) == 1 {
		target, err := c.workspaces.StatCheckout(r.Context(), actor, p.WorkspaceRecID, workRel, rels[0], absLocal)
		if writePathErr(w, err) {
			return
		}
		if !target.Dir {
			writeAttachment(w, target.Name, false)
			if typ := mime.TypeByExtension(filepath.Ext(target.Name)); typ != "" {
				w.Header().Set("Content-Type", typ)
			}
			http.ServeFile(w, r, target.Abs)
			return
		}
		serveCheckoutZip(w, r, []string{target.Abs}, target.Name)
		return
	}
	srcs := make([]string, 0, len(rels))
	for _, rel := range rels {
		target, err := c.workspaces.StatCheckout(r.Context(), actor, p.WorkspaceRecID, workRel, rel, absLocal)
		if writePathErr(w, err) {
			return
		}
		srcs = append(srcs, target.Abs)
	}
	name := strings.TrimSpace(p.ProjectName)
	if name == "" {
		name = "files"
	}
	serveCheckoutZip(w, r, srcs, name+"-files")
}

func collectDownloadPaths(q url.Values) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0)
	for _, key := range []string{"path", "path[]"} {
		for _, raw := range q[key] {
			p := strings.TrimSpace(raw)
			if p == "" {
				continue
			}
			if _, ok := seen[p]; ok {
				continue
			}
			seen[p] = struct{}{}
			out = append(out, p)
			if len(out) >= maxDownloadPaths {
				return out
			}
		}
	}
	return out
}

func serveCheckoutZip(w http.ResponseWriter, r *http.Request, srcs []string, name string) {
	tmp, err := os.CreateTemp("", "ap-checkout-*.zip")
	if writePathErr(w, err) {
		return
	}
	zipPath := tmp.Name()
	_ = tmp.Close()
	defer os.Remove(zipPath)
	if err := utils.ZipSources(srcs, zipPath, workspace.SkipNestedGit); err != nil {
		writePathErr(w, err)
		return
	}
	writeAttachment(w, name, true)
	w.Header().Set("Content-Type", "application/zip")
	http.ServeFile(w, r, zipPath)
}

// DeleteFile 删除工程检出目录内的文件或子目录。
func (c *Controller) DeleteFile(w http.ResponseWriter, r *http.Request) {
	actor := middleware.ActorFrom(r)
	p, err := c.projects.Get(r.Context(), actor, r.PathValue("id"))
	if httpx.WriteSvcErr(w, err) {
		return
	}
	rel := r.URL.Query().Get("path")
	err = c.workspaces.RemoveCheckout(r.Context(), actor, p.WorkspaceRecID, c.checkoutWorkRel(r.Context(), p), rel, c.checkoutAbs(r.Context(), p))
	c.audit.Note(r.Context(), actor, p.WorkspaceRecID, audit.ActionProjectDeleteFile, "project", p.RecID, "delete "+rel, err)
	if writePathErr(w, err) {
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func writeAttachment(w http.ResponseWriter, name string, zip bool) {
	name = filepath.Base(strings.ReplaceAll(strings.TrimSpace(name), "\\", "/"))
	if name == "" || name == "." || name == ".." {
		name = "download"
	}
	if zip && !strings.HasSuffix(strings.ToLower(name), ".zip") {
		name += ".zip"
	}
	ascii := strings.Map(func(r rune) rune {
		if r < 32 || r > 126 || r == '"' {
			return '_'
		}
		return r
	}, name)
	if ascii == "" {
		ascii = "download"
	}
	w.Header().Set("Content-Disposition", `attachment; filename="`+ascii+`"; filename*=UTF-8''`+url.PathEscape(name))
}

func writePathErr(w http.ResponseWriter, err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, workspace.ErrOutsideCheckout) || errors.Is(err, workspace.ErrNotFile) ||
		errors.Is(err, workspace.ErrNotDir) || errors.Is(err, workspace.ErrNotText) {
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
		return true
	}
	if os.IsNotExist(err) {
		httpx.WriteError(w, http.StatusNotFound, err.Error())
		return true
	}
	return httpx.WriteSvcErr(w, err)
}

// RemoteNote 读取推送目录下已有的 updateNote.txt，供界面预览追加。
func (c *Controller) RemoteNote(w http.ResponseWriter, r *http.Request) {
	var in struct {
		EnvRecID string `json:"env_rec_id"`
		PushPath string `json:"push_path"`
		FileName string `json:"file_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid body")
		return
	}
	actor := middleware.ActorFrom(r)
	p, err := c.projects.Get(r.Context(), actor, r.PathValue("id"))
	if httpx.WriteSvcErr(w, err) {
		return
	}
	if strings.TrimSpace(in.EnvRecID) == "" || strings.TrimSpace(in.PushPath) == "" {
		httpx.WriteError(w, http.StatusBadRequest, "env_rec_id and push_path required")
		return
	}
	row, cfg, err := c.envs.DecryptRow(r.Context(), in.EnvRecID)
	if httpx.WriteSvcErr(w, err) {
		return
	}
	if row.WorkspaceRecID != p.WorkspaceRecID {
		httpx.WriteError(w, http.StatusBadRequest, "workspace mismatch")
		return
	}
	kind, err := env.NormalizeKind(row.EnvKind)
	if httpx.WriteSvcErr(w, err) {
		return
	}
	if kind != "ftp" && kind != "sftp" && kind != "ssh" {
		httpx.WriteError(w, http.StatusBadRequest, "env must be ftp, sftp or ssh")
		return
	}
	exists, content, infoPath, err := readRemoteNote(kind, cfg, in.PushPath, in.FileName)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"exists":    exists,
		"content":   content,
		"info_path": infoPath,
	})
}

// Delete 删除工程。
func (c *Controller) Delete(w http.ResponseWriter, r *http.Request) {
	actor := middleware.ActorFrom(r)
	id := r.PathValue("id")
	p, _ := c.projects.Get(r.Context(), actor, id)
	err := c.projects.Delete(r.Context(), actor, id)
	c.audit.Note(r.Context(), actor, p.WorkspaceRecID, audit.ActionProjectDelete, "project", id, "delete "+p.ProjectName, err)
	if httpx.WriteSvcErr(w, err) {
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (c *Controller) sourceCtx(ctx context.Context, p project.Project) project.SourceContext {
	ws, err := c.workspaces.GetRaw(ctx, p.WorkspaceRecID)
	if err != nil {
		return project.SourceContextFrom(p, p.WorkspaceRecID, "", "", "")
	}
	return project.SourceContextFrom(p, ws.RecID, ws.WorkspaceCode, ws.WorkspaceName, ws.RootDir)
}

func (c *Controller) checkoutAbs(ctx context.Context, p project.Project) string {
	recipe := project.ParseRecipe(p.RecipeJSON)
	if abs := project.AbsLocalSourceCtx(recipe, c.sourceCtx(ctx, p)); abs != "" {
		return abs
	}
	return project.AbsLocalSource(recipe)
}

func (c *Controller) checkoutWorkRel(ctx context.Context, p project.Project) string {
	recipe := project.ParseRecipe(p.RecipeJSON)
	srcCtx := c.sourceCtx(ctx, p)
	if project.AbsLocalSourceCtx(recipe, srcCtx) != "" {
		return strings.TrimSpace(p.WorkRelPath)
	}
	rel := strings.TrimSpace(p.WorkRelPath)
	if rel == "" {
		return project.WorkRelCtx(p.ProjectName, recipe, srcCtx)
	}
	return project.ExpandSourceVars(rel, srcCtx, recipe)
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

func readRemoteNote(kind string, cfg map[string]any, pushPath, fileName string) (exists bool, content, infoPath string, err error) {
	pushDir, _ := utils.PushLayout(pushPath, fileName)
	infoPath = utils.RemoteJoin(pushDir, utils.UpdateNoteName)
	switch kind {
	case "sftp", "ssh":
		exists, raw, err := hostssh.ReadRemote(hostssh.MapConfig(cfg), infoPath)
		return exists, string(raw), infoPath, err
	case "oss":
		sess, err := hostoss.Open(hostoss.MapConfig(cfg))
		if err != nil {
			return false, "", infoPath, err
		}
		defer sess.Close()
		exists, err = sess.Exists(infoPath)
		if err != nil || !exists {
			return exists, "", infoPath, err
		}
		raw, err := sess.Download(infoPath)
		return true, string(raw), infoPath, err
	default:
		exists, raw, err := ftp.ReadRemote(ftp.MapConfig(cfg), infoPath)
		return exists, string(raw), infoPath, err
	}
}
