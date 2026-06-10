package workspace

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"auto-pack/backend/internal/httpx"
	"auto-pack/backend/internal/repository"
	"auto-pack/backend/pkg/audit"
	"auto-pack/backend/pkg/projectcfg"

	"github.com/google/uuid"
)

// Handler manages /api/workspaces CRUD.
type Handler struct {
	repo         *repository.WorkspaceRepository
	projects     *repository.ProjectRepository
	builds       *repository.BuildRepository
	releases     *repository.WorkspaceReleaseRepository
	scheduler    BuildSchedulerReloader
	releaseGuard ReleaseActivityChecker
	audit        *audit.Recorder
	dataDir      string
}

func NewHandler(
	repo *repository.WorkspaceRepository,
	projects *repository.ProjectRepository,
	builds *repository.BuildRepository,
	releases *repository.WorkspaceReleaseRepository,
	scheduler BuildSchedulerReloader,
	releaseGuard ReleaseActivityChecker,
	dataDir string,
	rec *audit.Recorder,
) *Handler {
	return &Handler{
		repo:         repo,
		projects:     projects,
		builds:       builds,
		releases:     releases,
		scheduler:    scheduler,
		releaseGuard: releaseGuard,
		audit:        rec,
		dataDir:      dataDir,
	}
}

type workspaceItem struct {
	repository.PackWorkspace
	EffectiveRootDir string `json:"effective_root_dir"`
}

func (h *Handler) toItem(ws *repository.PackWorkspace) workspaceItem {
	return workspaceItem{
		PackWorkspace:    *ws,
		EffectiveRootDir: projectcfg.ResolveWorkspaceRoot(ws.RootDir, h.dataDir, ws.WorkspaceCode),
	}
}

// Register mounts workspace routes.
func Register(mux *http.ServeMux, h *Handler, requireAuth func(http.Handler) http.Handler, apiBase string) {
	base := apiBase + "/workspaces"
	mux.Handle("GET "+base, requireAuth(http.HandlerFunc(h.list)))
	mux.Handle("POST "+base, requireAuth(http.HandlerFunc(h.create)))
	mux.Handle("POST "+base+"/clone", requireAuth(http.HandlerFunc(h.clone)))
	mux.Handle("GET "+base+"/{recID}", requireAuth(http.HandlerFunc(h.get)))
	mux.Handle("PUT "+base+"/{recID}", requireAuth(http.HandlerFunc(h.update)))
	mux.Handle("DELETE "+base+"/{recID}", requireAuth(http.HandlerFunc(h.delete)))
	mux.Handle("POST "+base+"/{recID}/purge", requireAuth(http.HandlerFunc(h.purge)))
	mux.Handle("GET "+base+"/{recID}/maven-dependency-analysis", requireAuth(http.HandlerFunc(h.mavenDependencyAnalysis)))
}

func (h *Handler) list(w http.ResponseWriter, _ *http.Request) {
	items, err := h.repo.List()
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	out := make([]workspaceItem, 0, len(items))
	for i := range items {
		out = append(out, h.toItem(&items[i]))
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": out})
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	item, err := h.repo.FindByRecID(r.PathValue("recID"))
	if err != nil {
		writeRepoError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, h.toItem(item))
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var body wsPayload
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	item, err := body.toModel("")
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	item.RecID = uuid.New().String()
	if item.IsDefault == 1 {
		_ = h.repo.ClearDefault(item.RecID)
	}
	if err := h.repo.Insert(item); err != nil {
		writeRepoError(w, err)
		return
	}
	created, _ := h.repo.FindByRecID(item.RecID)
	h.audit.Record(r.Context(), r, audit.Entry{
		Action: audit.ActionCreate, Module: audit.ModuleWorkspace, TargetType: "workspace",
		TargetID: created.RecID, TargetName: created.WorkspaceName,
		Summary: "创建工作空间 " + created.WorkspaceName, Success: true,
	})
	httpx.WriteJSON(w, http.StatusCreated, h.toItem(created))
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	recID := r.PathValue("recID")
	if _, err := h.repo.FindByRecID(recID); err != nil {
		writeRepoError(w, err)
		return
	}
	var body wsPayload
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	item, err := body.toModel(recID)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	if item.IsDefault == 1 {
		_ = h.repo.ClearDefault(item.RecID)
	}
	if err := h.repo.Update(item); err != nil {
		writeRepoError(w, err)
		return
	}
	updated, _ := h.repo.FindByRecID(recID)
	h.audit.Record(r.Context(), r, audit.Entry{
		Action: audit.ActionUpdate, Module: audit.ModuleWorkspace, TargetType: "workspace",
		TargetID: updated.RecID, TargetName: updated.WorkspaceName,
		Summary: "更新工作空间 " + updated.WorkspaceName, Success: true,
	})
	httpx.WriteJSON(w, http.StatusOK, h.toItem(updated))
}

func (h *Handler) delete(w http.ResponseWriter, r *http.Request) {
	recID := r.PathValue("recID")
	if recID == repository.DefaultWorkspaceRecID {
		httpx.WriteError(w, http.StatusBadRequest, "cannot delete default workspace")
		return
	}
	ws, err := h.repo.FindByRecID(recID)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	if ws.IsDefault == 1 {
		httpx.WriteError(w, http.StatusBadRequest, "cannot delete default workspace")
		return
	}
	if err := h.repo.Delete(recID); err != nil {
		writeRepoError(w, err)
		return
	}
	h.audit.Record(r.Context(), r, audit.Entry{
		Action: audit.ActionDelete, Module: audit.ModuleWorkspace, TargetType: "workspace",
		TargetID: recID, TargetName: ws.WorkspaceName,
		Summary: "删除工作空间 " + ws.WorkspaceName, Success: true,
	})
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"message": "deleted"})
}

type wsPayload struct {
	WorkspaceCode string `json:"workspace_code"`
	WorkspaceName string `json:"workspace_name"`
	RootDir       string `json:"root_dir"`
	IsDefault     int    `json:"is_default"`
	IsEnabled     int    `json:"is_enabled"`
	SortNum       int    `json:"sort_num"`
	Remark        string `json:"remark"`
}

func (p *wsPayload) toModel(recID string) (*repository.PackWorkspace, error) {
	name := strings.TrimSpace(p.WorkspaceName)
	if name == "" {
		return nil, errors.New("workspace_name is required")
	}
	if err := projectcfg.ValidateWorkspaceCode(p.WorkspaceCode); err != nil {
		return nil, err
	}
	isDefault := 0
	if p.IsDefault == 1 {
		isDefault = 1
	}
	isEnabled := 1
	if p.IsEnabled == 0 {
		isEnabled = 0
	}
	return &repository.PackWorkspace{
		RecID:         recID,
		WorkspaceCode: strings.TrimSpace(p.WorkspaceCode),
		WorkspaceName: name,
		RootDir:       strings.TrimSpace(p.RootDir),
		IsDefault:     isDefault,
		IsEnabled:     isEnabled,
		SortNum:       p.SortNum,
		Remark:        strings.TrimSpace(p.Remark),
	}, nil
}

func writeRepoError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, repository.ErrWorkspaceNotFound):
		httpx.WriteError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, repository.ErrWorkspaceCodeExists):
		httpx.WriteError(w, http.StatusConflict, err.Error())
	case errors.Is(err, repository.ErrWorkspaceHasProjects):
		httpx.WriteError(w, http.StatusConflict, err.Error())
	default:
		httpx.WriteError(w, http.StatusInternalServerError, err.Error())
	}
}
