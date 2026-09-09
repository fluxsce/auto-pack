package workspace

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"auto-pack/backend/internal/httpx"
	"auto-pack/backend/internal/repository"
	"auto-pack/backend/pkg/audit"
	"auto-pack/backend/pkg/projectcfg"

	"github.com/google/uuid"
)

// BuildSchedulerReloader reloads cron jobs after workspace project clone.
type BuildSchedulerReloader interface {
	ReloadBuildScheduler() error
}

// ReleaseActivityChecker reports whether a workspace auto-release is in progress.
type ReleaseActivityChecker interface {
	WorkspaceReleaseActive(workspaceRecID string) bool
}

type clonePayload struct {
	SourceRecID string `json:"source_rec_id"`
	wsPayload
}

type cloneResponse struct {
	workspaceItem
	ProjectsCloned int `json:"projects_cloned"`
}

func (h *Handler) clone(w http.ResponseWriter, r *http.Request) {
	var body clonePayload
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	sourceID := strings.TrimSpace(body.SourceRecID)
	if sourceID == "" {
		httpx.WriteError(w, http.StatusBadRequest, "source_rec_id is required")
		return
	}
	if h.projects == nil {
		httpx.WriteError(w, http.StatusInternalServerError, "project repository not configured")
		return
	}
	if _, err := h.repo.FindByRecID(sourceID); err != nil {
		writeRepoError(w, err)
		return
	}
	dest, err := body.wsPayload.toModel("")
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	dest.RecID = uuid.New().String()
	dest.IsDefault = 0
	projects, err := h.projects.List(sourceID, "")
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	idMap := make(map[string]string, len(projects))
	for _, p := range projects {
		idMap[p.RecID] = uuid.New().String()
	}
	cloned := make([]repository.PackProject, 0, len(projects))
	for _, src := range projects {
		newID := idMap[src.RecID]
		name, version, repoURL, branch, remark := projectcfg.NormalizeProjectSCM(
			src.ProjectName, src.ProjectVersion, src.RepoURL, src.RepoBranch, src.Remark,
		)
		if err := projectcfg.ValidateProjectName(name); err != nil {
			httpx.WriteError(w, http.StatusBadRequest, err.Error())
			return
		}
		if err := projectcfg.ValidateVersion(version); err != nil {
			httpx.WriteError(w, http.StatusBadRequest, err.Error())
			return
		}
		cfgJSON, err := projectcfg.RemapBuildDependsOn(src.ConfigJSON, idMap)
		if err != nil {
			httpx.WriteError(w, http.StatusBadRequest, err.Error())
			return
		}
		cfg, err := projectcfg.ParseConfigJSON(cfgJSON)
		if err != nil {
			httpx.WriteError(w, http.StatusBadRequest, err.Error())
			return
		}
		if err := projectcfg.ValidateBuildTriggers(newID, cfg.BuildTriggers); err != nil {
			httpx.WriteError(w, http.StatusBadRequest, err.Error())
			return
		}
		normalized, err := projectcfg.NormalizeConfigJSON(cfgJSON)
		if err != nil {
			httpx.WriteError(w, http.StatusBadRequest, err.Error())
			return
		}
		cloned = append(cloned, repository.PackProject{
			RecID:          newID,
			WorkspaceRecID: dest.RecID,
			ProjectName:    name,
			ProjectVersion: version,
			ProjectType:    src.ProjectType,
			RepoURL:        repoURL,
			RepoBranch:     branch,
			WorkDir:        projectcfg.SuggestWorkDir(name),
			ConfigJSON:     normalized,
			GitEnvRecID:    strings.TrimSpace(src.GitEnvRecID),
			JdkEnvRecID:    strings.TrimSpace(src.JdkEnvRecID),
			MavenEnvRecID:  strings.TrimSpace(src.MavenEnvRecID),
			NodejsEnvRecID: strings.TrimSpace(src.NodejsEnvRecID),
			DeployEnvRecID: strings.TrimSpace(src.DeployEnvRecID),
			IsEnabled:      src.IsEnabled,
			SortNum:        src.SortNum,
			Remark:         remark,
		})
	}
	if err := h.repo.Insert(dest); err != nil {
		writeRepoError(w, err)
		return
	}
	for i := range cloned {
		if err := h.projects.Insert(&cloned[i]); err != nil {
			for j := 0; j < i; j++ {
				_ = h.projects.Delete(cloned[j].RecID)
			}
			_ = h.repo.Delete(dest.RecID)
			writeProjectCloneError(w, err)
			return
		}
	}
	if h.scheduler != nil {
		_ = h.scheduler.ReloadBuildScheduler()
	}
	created, _ := h.repo.FindByRecID(dest.RecID)
	h.audit.Record(r.Context(), r, audit.Entry{
		Action: audit.ActionClone, Module: audit.ModuleWorkspace, TargetType: "workspace",
		TargetID: created.RecID, TargetName: created.WorkspaceName,
		Summary: "克隆工作空间 " + created.WorkspaceName,
		Detail:  "source=" + sourceID + ", projects=" + strconv.Itoa(len(cloned)),
		Success: true,
	})
	httpx.WriteJSON(w, http.StatusCreated, cloneResponse{
		workspaceItem:  h.toItem(created),
		ProjectsCloned: len(cloned),
	})
}

func writeProjectCloneError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, repository.ErrProjectVersionExists):
		httpx.WriteError(w, http.StatusConflict, err.Error())
	default:
		httpx.WriteError(w, http.StatusInternalServerError, err.Error())
	}
}
