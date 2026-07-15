package workspace

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"auto-pack/backend/internal/httpx"
	"auto-pack/backend/internal/repository"
	"auto-pack/backend/pkg/audit"
	"auto-pack/backend/pkg/workspacebundle"
)

type importRequest struct {
	Mode                 string                 `json:"mode"`
	TargetWorkspaceRecID string                 `json:"target_workspace_rec_id"`
	WorkspaceCode        string                 `json:"workspace_code"`
	WorkspaceName        string                 `json:"workspace_name"`
	RootDir              string                 `json:"root_dir"`
	Bundle               workspacebundle.Bundle `json:"bundle"`
}

type importResponse struct {
	workspaceItem
	ProjectsImported int    `json:"projects_imported"`
	Mode             string `json:"mode"`
}

func (h *Handler) export(w http.ResponseWriter, r *http.Request) {
	recID := r.PathValue("recID")
	ws, err := h.repo.FindByRecID(recID)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	projects, err := h.projects.List(recID, "")
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	bundle := workspacebundle.Export(ws, projects)
	data, err := json.MarshalIndent(bundle, "", "  ")
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	filename := exportFilename(ws.WorkspaceCode)
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	h.audit.Record(r.Context(), r, audit.Entry{
		Action: audit.ActionVerify, Module: audit.ModuleWorkspace, TargetType: "workspace",
		TargetID: ws.RecID, TargetName: ws.WorkspaceName,
		Summary: "导出工作空间 " + ws.WorkspaceName,
		Detail:  "projects=" + strconv.Itoa(len(projects)),
		Success: true,
	})
	_, _ = w.Write(data)
}

func exportFilename(workspaceCode string) string {
	code := strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_', r == '.':
			return r
		default:
			return '-'
		}
	}, strings.TrimSpace(workspaceCode))
	code = strings.Trim(code, "-.")
	if code == "" {
		code = "workspace"
	}
	ts := time.Now().UTC().Format("20060102-150405")
	return code + "-autopack-" + ts + ".json"
}

func (h *Handler) importBundle(w http.ResponseWriter, r *http.Request) {
	var body importRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	mode := strings.ToLower(strings.TrimSpace(body.Mode))
	switch mode {
	case "new", "replace":
	default:
		httpx.WriteError(w, http.StatusBadRequest, "mode must be new or replace")
		return
	}
	if err := body.Bundle.Validate(); err != nil {
		writeBundleError(w, err)
		return
	}
	switch mode {
	case "new":
		h.importAsNewWorkspace(w, r, body)
	case "replace":
		h.importReplaceWorkspace(w, r, body)
	}
}

func (h *Handler) importAsNewWorkspace(w http.ResponseWriter, r *http.Request, body importRequest) {
	ws, err := body.Bundle.BuildWorkspace(workspacebundle.NewWorkspaceOptions{
		WorkspaceCode: body.WorkspaceCode,
		WorkspaceName: body.WorkspaceName,
		RootDir:       body.RootDir,
	})
	if err != nil {
		writeBundleError(w, err)
		return
	}
	projects, err := body.Bundle.BuildProjects(ws.RecID)
	if err != nil {
		writeBundleError(w, err)
		return
	}
	if err := h.repo.Insert(ws); err != nil {
		writeRepoError(w, err)
		return
	}
	if err := h.insertImportedProjects(projects); err != nil {
		_ = h.repo.DeleteUnchecked(ws.RecID)
		writeProjectCloneError(w, err)
		return
	}
	if h.scheduler != nil {
		_ = h.scheduler.ReloadBuildScheduler()
	}
	created, _ := h.repo.FindByRecID(ws.RecID)
	h.audit.Record(r.Context(), r, audit.Entry{
		Action: audit.ActionCreate, Module: audit.ModuleWorkspace, TargetType: "workspace",
		TargetID: created.RecID, TargetName: created.WorkspaceName,
		Summary: "导入工作空间 " + created.WorkspaceName,
		Detail:  "mode=new, projects=" + strconv.Itoa(len(projects)),
		Success: true,
	})
	httpx.WriteJSON(w, http.StatusCreated, importResponse{
		workspaceItem:    h.toItem(created),
		ProjectsImported: len(projects),
		Mode:             "new",
	})
}

func (h *Handler) importReplaceWorkspace(w http.ResponseWriter, r *http.Request, body importRequest) {
	targetID := strings.TrimSpace(body.TargetWorkspaceRecID)
	if targetID == "" {
		httpx.WriteError(w, http.StatusBadRequest, "target_workspace_rec_id is required")
		return
	}
	if _, err := h.repo.FindByRecID(targetID); err != nil {
		writeRepoError(w, err)
		return
	}
	if err := h.guardWorkspaceImport(targetID); err != nil {
		httpx.WriteError(w, http.StatusConflict, err.Error())
		return
	}
	projects, err := body.Bundle.BuildProjects(targetID)
	if err != nil {
		writeBundleError(w, err)
		return
	}
	if err := h.builds.DeleteByWorkspace(targetID); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if _, err := h.projects.DeleteByWorkspace(targetID); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := h.insertImportedProjects(projects); err != nil {
		writeProjectCloneError(w, err)
		return
	}
	if h.scheduler != nil {
		_ = h.scheduler.ReloadBuildScheduler()
	}
	updated, _ := h.repo.FindByRecID(targetID)
	h.audit.Record(r.Context(), r, audit.Entry{
		Action: audit.ActionUpdate, Module: audit.ModuleWorkspace, TargetType: "workspace",
		TargetID: updated.RecID, TargetName: updated.WorkspaceName,
		Summary: "导入并替换工作空间工程 " + updated.WorkspaceName,
		Detail:  "mode=replace, projects=" + strconv.Itoa(len(projects)),
		Success: true,
	})
	httpx.WriteJSON(w, http.StatusOK, importResponse{
		workspaceItem:    h.toItem(updated),
		ProjectsImported: len(projects),
		Mode:             "replace",
	})
}

func (h *Handler) guardWorkspaceImport(workspaceRecID string) error {
	if h.releaseGuard != nil && h.releaseGuard.WorkspaceReleaseActive(workspaceRecID) {
		return errors.New("auto release is running for this workspace")
	}
	if runningRelease, err := h.releases.FindRunningByWorkspace(workspaceRecID); err != nil {
		return err
	} else if runningRelease != nil {
		return errors.New("auto release is running for this workspace")
	}
	if runningBuild, err := h.builds.FindRunningByWorkspace(workspaceRecID); err != nil {
		return err
	} else if runningBuild != nil {
		return errors.New("cannot import while a build is running")
	}
	return nil
}

func (h *Handler) insertImportedProjects(projects []repository.PackProject) error {
	for i := range projects {
		if err := h.projects.Insert(&projects[i]); err != nil {
			for j := 0; j < i; j++ {
				_ = h.projects.Delete(projects[j].RecID)
			}
			return err
		}
	}
	return nil
}

func writeBundleError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, workspacebundle.ErrUnsupportedFormat),
		errors.Is(err, workspacebundle.ErrBundleEmpty),
		errors.Is(err, workspacebundle.ErrWorkspaceRequired):
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
	default:
		msg := err.Error()
		if strings.Contains(msg, "project[") || strings.Contains(msg, "workspace") || strings.Contains(msg, "project_type") {
			httpx.WriteError(w, http.StatusBadRequest, msg)
			return
		}
		httpx.WriteError(w, http.StatusBadRequest, msg)
	}
}
