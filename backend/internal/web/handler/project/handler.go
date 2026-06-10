package project

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"auto-pack/backend/internal/httpx"
	"auto-pack/backend/internal/repository"
	"auto-pack/backend/pkg/audit"
	"auto-pack/backend/pkg/logger"
	"auto-pack/backend/pkg/mavendeps"
	"auto-pack/backend/pkg/packrun"
	"auto-pack/backend/pkg/projectcfg"
	"auto-pack/backend/pkg/projectrelease"

	"github.com/google/uuid"
)

type projectListItem struct {
	repository.PackProject
	LastBuildStatus string     `json:"last_build_status,omitempty"`
	LastBuildNum    int        `json:"last_build_num,omitempty"`
	LastBuildAt     *time.Time `json:"last_build_at,omitempty"`
}

// Handler manages /api/projects CRUD and build/workspace operations.
type Handler struct {
	projects     *repository.ProjectRepository
	workspaces   *repository.WorkspaceRepository
	env          *repository.EnvRepository
	builds       *repository.BuildRepository
	releases     *repository.WorkspaceReleaseRepository
	runner       *packrun.Runner
	releaseCoord *projectrelease.Coordinator
	audit        *audit.Recorder
	dataDir      string
}

func NewHandler(
	projects *repository.ProjectRepository,
	workspaces *repository.WorkspaceRepository,
	env *repository.EnvRepository,
	builds *repository.BuildRepository,
	releases *repository.WorkspaceReleaseRepository,
	dataDir string,
	rec *audit.Recorder,
) *Handler {
	h := &Handler{
		projects:     projects,
		workspaces:   workspaces,
		env:          env,
		builds:       builds,
		releases:     releases,
		runner:       packrun.NewRunner(projects, workspaces, env, builds, dataDir),
		releaseCoord: projectrelease.NewCoordinator(),
		audit:        rec,
		dataDir:      dataDir,
	}
	if n, err := builds.RecoverInterrupted(""); err != nil {
		logger.Warn("recover interrupted builds failed", "err", err.Error())
	} else if n > 0 {
		logger.Info("recovered interrupted builds", "count", n)
	}
	if n, err := releases.RecoverInterrupted(""); err != nil {
		logger.Warn("recover interrupted releases failed", "err", err.Error())
	} else if n > 0 {
		logger.Info("recovered interrupted releases", "count", n)
	}
	if err := h.runner.StartScheduler(); err != nil {
		logger.Warn("build scheduler start failed", "err", err.Error())
	}
	go h.runner.PruneAllProjectBuildLogs()
	return h
}

// Register mounts project routes (requires auth wrapper).
func Register(mux *http.ServeMux, h *Handler, requireAuth func(http.Handler) http.Handler, apiBase string) {
	base := apiBase + "/projects"
	mux.Handle("GET "+base, requireAuth(http.HandlerFunc(h.list)))
	mux.Handle("POST "+base, requireAuth(http.HandlerFunc(h.create)))
	mux.Handle("GET "+base+"/{recID}", requireAuth(http.HandlerFunc(h.get)))
	mux.Handle("PUT "+base+"/{recID}", requireAuth(http.HandlerFunc(h.update)))
	mux.Handle("DELETE "+base+"/{recID}", requireAuth(http.HandlerFunc(h.delete)))
	mux.Handle("GET "+base+"/{recID}/maven-dependency-analysis", requireAuth(http.HandlerFunc(h.mavenDependencyAnalysis)))
	mux.Handle("GET "+base+"/{recID}/builds", requireAuth(http.HandlerFunc(h.listBuilds)))
	mux.Handle("POST "+base+"/{recID}/builds", requireAuth(http.HandlerFunc(h.triggerBuild)))
	mux.Handle("GET "+base+"/{recID}/builds/{buildID}", requireAuth(http.HandlerFunc(h.getBuild)))
	mux.Handle("GET "+base+"/{recID}/builds/{buildID}/log", requireAuth(http.HandlerFunc(h.getBuildLog)))
	mux.Handle("GET "+base+"/{recID}/workspace", requireAuth(http.HandlerFunc(h.browseWorkspace)))
	mux.Handle("DELETE "+base+"/{recID}/workspace", requireAuth(http.HandlerFunc(h.deleteWorkspace)))
	mux.Handle("POST "+base+"/{recID}/workspace/ftp-push", requireAuth(http.HandlerFunc(h.workspaceFtpPush)))
	mux.Handle("GET "+base+"/{recID}/workspace/ftp-remote-note", requireAuth(http.HandlerFunc(h.workspaceFtpRemoteNote)))
	mux.Handle("GET "+base+"/{recID}/workspace/file", requireAuth(http.HandlerFunc(h.readWorkspaceFile)))
	mux.Handle("GET "+base+"/{recID}/workspace/download", requireAuth(http.HandlerFunc(h.downloadWorkspaceEntry)))
	mux.Handle("GET "+apiBase+"/project/types", requireAuth(http.HandlerFunc(h.types)))
	mux.Handle("POST "+base+"/batch-repo-branch", requireAuth(http.HandlerFunc(h.batchRepoBranch)))
	mux.Handle("POST "+base+"/batch-project-version", requireAuth(http.HandlerFunc(h.batchProjectVersion)))
	mux.Handle("POST "+base+"/batch-post-build-email", requireAuth(http.HandlerFunc(h.batchPostBuildEmail)))
	mux.Handle("POST "+base+"/batch-build-steps", requireAuth(http.HandlerFunc(h.batchBuildSteps)))
	mux.Handle("POST "+base+"/cron-preview", requireAuth(http.HandlerFunc(h.previewCron)))
	mux.Handle("POST "+base+"/batch-auto-release", requireAuth(http.HandlerFunc(h.batchAutoRelease)))
	mux.Handle("GET "+base+"/batch-auto-release/defaults", requireAuth(http.HandlerFunc(h.batchAutoReleaseDefaults)))
	mux.Handle("GET "+base+"/batch-auto-release/status", requireAuth(http.HandlerFunc(h.batchAutoReleaseStatus)))
	mux.Handle("PUT "+base+"/batch-auto-release/post-options", requireAuth(http.HandlerFunc(h.batchAutoReleasePostOptions)))
	mux.Handle("GET "+base+"/batch-auto-release/history", requireAuth(http.HandlerFunc(h.listBatchAutoReleaseHistory)))
	mux.Handle("GET "+base+"/batch-auto-release/history/{releaseID}/log", requireAuth(http.HandlerFunc(h.getBatchAutoReleaseLog)))
}

// ReloadBuildScheduler refreshes cron registrations after bulk project changes.
func (h *Handler) ReloadBuildScheduler() error {
	if h.runner == nil {
		return nil
	}
	return h.runner.Scheduler().Reload()
}

// WorkspaceReleaseActive reports whether an auto-release job is running for the workspace.
func (h *Handler) WorkspaceReleaseActive(workspaceRecID string) bool {
	st := h.releaseCoord.Status(workspaceRecID)
	return st.Status == projectrelease.JobStatusRunning && st.WorkspaceRecID == workspaceRecID
}

// AnalyzeMavenWorkspaceAll scans all Maven projects in a workspace (for workspace-level analysis API).
func (h *Handler) AnalyzeMavenWorkspaceAll(
	workspaceRecID, settingsXML string,
) (*mavendeps.WorkspaceAnalysisResult, []mavendeps.Module, error) {
	return h.runner.AnalyzeMavenWorkspaceAll(workspaceRecID, settingsXML)
}

// SyncWorkspaceSources clones or updates git worktrees for all projects in a workspace (blocking).
func (h *Handler) SyncWorkspaceSources(ctx context.Context, workspaceRecID string) (*packrun.WorkspaceSyncResult, error) {
	return h.runner.SyncWorkspaceSources(ctx, workspaceRecID)
}

// StartWorkspaceSourceSync begins async git pre-sync for dependency analysis.
func (h *Handler) StartWorkspaceSourceSync(workspaceRecID, workspaceName string) error {
	return h.runner.StartWorkspaceSourceSync(workspaceRecID, workspaceName)
}

// WorkspaceSourceSyncStatus returns async pre-sync progress.
func (h *Handler) WorkspaceSourceSyncStatus(workspaceRecID string) packrun.WorkspaceSourceSyncJob {
	return h.runner.WorkspaceSourceSyncStatus(workspaceRecID)
}

func (h *Handler) types(w http.ResponseWriter, _ *http.Request) {
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"types": projectcfg.AllTypes})
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	workspaceRecID := strings.TrimSpace(r.URL.Query().Get("workspace_rec_id"))
	projectType := strings.TrimSpace(r.URL.Query().Get("project_type"))
	items, err := h.projects.List(workspaceRecID, projectType)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	latest, err := h.builds.LatestByWorkspace(workspaceRecID)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	out := make([]projectListItem, 0, len(items))
	for _, p := range items {
		entry := projectListItem{PackProject: p}
		if b, ok := latest[p.RecID]; ok {
			entry.LastBuildStatus = b.Status
			entry.LastBuildNum = b.BuildNum
			if b.FinishedAt != nil {
				entry.LastBuildAt = b.FinishedAt
			} else {
				entry.LastBuildAt = &b.CreatedAt
			}
		}
		out = append(out, entry)
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": out})
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	recID := r.PathValue("recID")
	item, err := h.projects.FindByRecID(recID)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, item)
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var body projectPayload
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	item, err := body.toModel(h, "")
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	item.RecID = uuid.New().String()
	if err := h.projects.Insert(item); err != nil {
		writeRepoError(w, err)
		return
	}
	created, _ := h.projects.FindByRecID(item.RecID)
	_ = h.runner.Scheduler().ReloadProject(item.RecID)
	h.audit.Record(r.Context(), r, audit.Entry{
		Action: audit.ActionCreate, Module: audit.ModuleProject, TargetType: "project",
		TargetID: created.RecID, TargetName: created.ProjectName,
		Summary: "创建工程 " + created.ProjectName, Success: true,
	})
	httpx.WriteJSON(w, http.StatusCreated, created)
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	recID := r.PathValue("recID")
	if _, err := h.projects.FindByRecID(recID); err != nil {
		writeRepoError(w, err)
		return
	}
	var body projectPayload
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	item, err := body.toModel(h, recID)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := h.projects.Update(item); err != nil {
		writeRepoError(w, err)
		return
	}
	updated, _ := h.projects.FindByRecID(recID)
	_ = h.runner.Scheduler().ReloadProject(recID)
	h.audit.Record(r.Context(), r, audit.Entry{
		Action: audit.ActionUpdate, Module: audit.ModuleProject, TargetType: "project",
		TargetID: updated.RecID, TargetName: updated.ProjectName,
		Summary: "更新工程 " + updated.ProjectName, Success: true,
	})
	httpx.WriteJSON(w, http.StatusOK, updated)
}

func (h *Handler) delete(w http.ResponseWriter, r *http.Request) {
	recID := r.PathValue("recID")
	existing, err := h.projects.FindByRecID(recID)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	if err := h.projects.Delete(recID); err != nil {
		writeRepoError(w, err)
		return
	}
	_ = h.runner.Scheduler().Reload()
	h.audit.Record(r.Context(), r, audit.Entry{
		Action: audit.ActionDelete, Module: audit.ModuleProject, TargetType: "project",
		TargetID: recID, TargetName: existing.ProjectName,
		Summary: "删除工程 " + existing.ProjectName, Success: true,
	})
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"message": "deleted"})
}

type projectPayload struct {
	WorkspaceRecID string `json:"workspace_rec_id"`
	ProjectName    string `json:"project_name"`
	ProjectVersion string `json:"project_version"`
	ProjectType    string `json:"project_type"`
	RepoURL        string `json:"repo_url"`
	RepoBranch     string `json:"repo_branch"`
	WorkDir        string `json:"work_dir"`
	ConfigJSON     string `json:"config_json"`
	GitEnvRecID    string `json:"git_env_rec_id"`
	JdkEnvRecID    string `json:"jdk_env_rec_id"`
	MavenEnvRecID  string `json:"maven_env_rec_id"`
	NodejsEnvRecID string `json:"nodejs_env_rec_id"`
	DeployEnvRecID string `json:"deploy_env_rec_id"`
	IsEnabled      int    `json:"is_enabled"`
	SortNum        int    `json:"sort_num"`
	Remark         string `json:"remark"`
}

func (p *projectPayload) toModel(h *Handler, recID string) (*repository.PackProject, error) {
	name, version, repoURL, branch, remark := projectcfg.NormalizeProjectSCM(
		p.ProjectName, p.ProjectVersion, p.RepoURL, p.RepoBranch, p.Remark,
	)
	if err := projectcfg.ValidateProjectName(name); err != nil {
		return nil, err
	}
	projectType := strings.TrimSpace(strings.ToLower(p.ProjectType))
	if err := projectcfg.ValidateVersion(version); err != nil {
		return nil, err
	}
	wsRecID := strings.TrimSpace(p.WorkspaceRecID)
	if wsRecID == "" {
		def, err := h.workspaces.FindDefault()
		if err != nil {
			return nil, errors.New("workspace_rec_id is required")
		}
		wsRecID = def.RecID
	} else if _, err := h.workspaces.FindByRecID(wsRecID); err != nil {
		return nil, errors.New("workspace not found")
	}
	if projectType == "" {
		projectType = "custom"
	}
	if !projectcfg.IsValidType(projectType) {
		return nil, errors.New("invalid project_type")
	}
	cfgJSON, err := projectcfg.NormalizeConfigJSON(strings.TrimSpace(p.ConfigJSON))
	if err != nil {
		return nil, err
	}
	cfg, err := projectcfg.ParseConfigJSON(cfgJSON)
	if err != nil {
		return nil, err
	}
	if err := projectcfg.ValidateBuildTriggers(recID, cfg.BuildTriggers); err != nil {
		return nil, err
	}
	for _, depID := range cfg.BuildTriggers.DependsOn {
		dep, err := h.projects.FindByRecID(strings.TrimSpace(depID))
		if err != nil {
			return nil, errors.New("build dependency project not found")
		}
		if dep.WorkspaceRecID != wsRecID {
			return nil, errors.New("build dependencies must be in the same workspace")
		}
	}
	if err := h.validateEnvRefs(
		p.GitEnvRecID, "git",
		p.JdkEnvRecID, "jdk",
		p.MavenEnvRecID, "maven",
		p.NodejsEnvRecID, "nodejs",
		p.DeployEnvRecID, "",
	); err != nil {
		return nil, err
	}
	isEnabled := 1
	if p.IsEnabled == 0 {
		isEnabled = 0
	}
	// Work path is under workspace root: {slug(name)}
	workDir := projectcfg.SuggestWorkDir(name)
	return &repository.PackProject{
		RecID:          recID,
		WorkspaceRecID: wsRecID,
		ProjectName:    name,
		ProjectVersion: version,
		ProjectType:    projectType,
		RepoURL:        repoURL,
		RepoBranch:     branch,
		WorkDir:        workDir,
		ConfigJSON:     cfgJSON,
		GitEnvRecID:    strings.TrimSpace(p.GitEnvRecID),
		JdkEnvRecID:    strings.TrimSpace(p.JdkEnvRecID),
		MavenEnvRecID:  strings.TrimSpace(p.MavenEnvRecID),
		NodejsEnvRecID: strings.TrimSpace(p.NodejsEnvRecID),
		DeployEnvRecID: strings.TrimSpace(p.DeployEnvRecID),
		IsEnabled:      isEnabled,
		SortNum:        p.SortNum,
		Remark:         remark,
	}, nil
}

func (h *Handler) validateEnvRefs(pairs ...string) error {
	for i := 0; i < len(pairs); i += 2 {
		recID := strings.TrimSpace(pairs[i])
		wantType := strings.TrimSpace(strings.ToLower(pairs[i+1]))
		if recID == "" {
			continue
		}
		item, err := h.env.FindByRecID(recID)
		if err != nil {
			if errors.Is(err, repository.ErrEnvNotFound) {
				return errors.New("linked environment not found")
			}
			return err
		}
		if item.IsEnabled != 1 {
			return errors.New("linked environment is disabled")
		}
		if wantType == "" {
			if !projectcfg.DeployEnvTypes[item.EnvType] {
				return errors.New("deploy environment must be ssh, sftp, or ftp")
			}
			continue
		}
		if item.EnvType != wantType {
			return errors.New("linked environment type mismatch")
		}
	}
	return nil
}

func writeRepoError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, repository.ErrProjectNotFound):
		httpx.WriteError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, repository.ErrProjectVersionExists):
		httpx.WriteError(w, http.StatusConflict, err.Error())
	default:
		httpx.WriteError(w, http.StatusInternalServerError, err.Error())
	}
}
