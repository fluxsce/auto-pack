package project

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"auto-pack/backend/internal/httpx"
	"auto-pack/backend/internal/repository"
	"auto-pack/backend/pkg/audit"
	"auto-pack/backend/pkg/envtypes"
	"auto-pack/backend/pkg/workspaceftppush"

	"github.com/google/uuid"
)

type workspaceFtpMergeSource struct {
	ProjectRecID string `json:"project_rec_id"`
	Path         string `json:"path"`
}

type workspaceFtpPushPayload struct {
	Path           string                          `json:"path"`
	Paths          []string                        `json:"paths,omitempty"`
	MergeZipRoot   string                          `json:"merge_zip_root,omitempty"`
	MergeSources   []workspaceFtpMergeSource       `json:"merge_sources,omitempty"`
	FTPEnvRecID    string                          `json:"ftp_env_rec_id"`
	PushPath       string                          `json:"push_path"`
	BackupPath     string                          `json:"backup_path"`
	Pusher         string                          `json:"pusher"`
	TaskRef        string                          `json:"task_ref"`
	Detail         string                          `json:"detail"`
	SkipUpdateNote bool                            `json:"skip_update_note"`
	PriorNotePaths []workspaceftppush.NotePathPair `json:"prior_note_paths,omitempty"`
}

type workspaceFtpPushStartResponse struct {
	TaskID string `json:"task_id"`
	Status string `json:"status"`
}

func (h *Handler) workspaceFtpPush(w http.ResponseWriter, r *http.Request) {
	projectRecID := r.PathValue("recID")
	if _, err := h.projects.FindByRecID(projectRecID); err != nil {
		writeRepoError(w, err)
		return
	}
	running, err := h.builds.FindRunningByProject(projectRecID)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if running != nil {
		httpx.WriteError(w, http.StatusConflict, "cannot push while a build is running")
		return
	}
	if active := h.ftpPushCoord.activeTask(projectRecID); active != nil {
		httpx.WriteError(w, http.StatusConflict, "ftp push already in progress")
		return
	}

	var body workspaceFtpPushPayload
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	rel := strings.TrimSpace(body.Path)
	mergeRoot := strings.TrimSpace(body.MergeZipRoot)
	if (rel == "" || rel == ".") && mergeRoot == "" && len(body.Paths) == 0 && len(body.MergeSources) == 0 {
		httpx.WriteError(w, http.StatusBadRequest, "path is required")
		return
	}
	envID := strings.TrimSpace(body.FTPEnvRecID)
	if envID == "" {
		httpx.WriteError(w, http.StatusBadRequest, "ftp_env_rec_id is required")
		return
	}
	item, err := h.env.FindByRecID(envID)
	if err != nil {
		if errors.Is(err, repository.ErrEnvNotFound) {
			httpx.WriteError(w, http.StatusNotFound, err.Error())
			return
		}
		httpx.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if item.EnvType != envtypes.TypeFTP {
		httpx.WriteError(w, http.StatusBadRequest, "environment must be ftp")
		return
	}
	ftpCfg, err := workspaceftppush.ParseFTPConfig(item.ConfigJSON)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	project, err := h.projects.FindByRecID(projectRecID)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	_, _, base, err := h.resolveProjectDir(projectRecID)
	if err != nil {
		writeProjectDirError(w, err)
		return
	}

	extraRels := normalizeRelPaths(body.Paths, rel)
	mergeSources, err := h.resolveFtpMergeSources(project, body.MergeSources)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	for _, src := range mergeSources {
		if running, err := h.builds.FindRunningByProject(src.projectRecID); err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, err.Error())
			return
		} else if running != nil {
			httpx.WriteError(w, http.StatusConflict, "cannot push while a build is running")
			return
		}
		if src.projectRecID != projectRecID {
			if active := h.ftpPushCoord.activeTask(src.projectRecID); active != nil {
				httpx.WriteError(w, http.StatusConflict, "ftp push already in progress")
				return
			}
		}
	}
	pushMergeSources := make([]workspaceftppush.SourcePath, 0, len(mergeSources))
	for _, src := range mergeSources {
		pushMergeSources = append(pushMergeSources, src.source)
	}

	objectLabel := rel
	if mergeRoot != "" {
		objectLabel = mergeRoot + ".zip"
	}

	taskID := uuid.New().String()
	h.ftpPushCoord.start(taskID, projectRecID, objectLabel, body.PushPath, func(ctx context.Context, appendLog func(format string, args ...any)) (*workspaceftppush.Result, error) {
		appendLog("开始 FTP 远程推送")
		if mergeRoot != "" {
			appendLog("对象：合并 %s（%d 个目录）", mergeRoot+".zip", 1+len(extraRels)+len(mergeSources))
		} else {
			appendLog("对象：%s", rel)
		}
		appendLog("FTP 环境：%s", item.EnvName)
		appendLog("推送路径：%s", strings.TrimSpace(body.PushPath))
		result, err := workspaceftppush.Push(workspaceftppush.Request{
			LocalBase:      base,
			RelativePath:   rel,
			RelativePaths:  extraRels,
			MergeZipRoot:   mergeRoot,
			MergeSources:   pushMergeSources,
			FTPConfig:      ftpCfg,
			PushPath:       body.PushPath,
			BackupPath:     body.BackupPath,
			Pusher:         body.Pusher,
			TaskRef:        body.TaskRef,
			Detail:         body.Detail,
			SkipUpdateNote: body.SkipUpdateNote,
			PriorNotePaths: body.PriorNotePaths,
			Logf:           appendLog,
		})
		if err != nil {
			appendLog("推送失败")
			appendLog("  %s", err.Error())
			h.audit.Record(r.Context(), r, audit.Entry{
				Action: audit.ActionUpdate, Module: audit.ModuleProject, TargetType: "project",
				TargetID: projectRecID, TargetName: project.ProjectName,
				Summary: "FTP 远程推送失败 " + objectLabel,
				Detail:  err.Error(),
				Success: false,
			})
			return nil, err
		}
		appendLog("推送成功")
		appendLog("  备份路径：%s", result.BackupPath)
		appendLog("  更新包路径：%s", result.UpdatePath)
		if body.SkipUpdateNote {
			appendLog("  说明文件：稍后合并写入")
		} else {
			appendLog("  说明文件：%s", result.InfoFilePath)
		}
		h.audit.Record(r.Context(), r, audit.Entry{
			Action: audit.ActionUpdate, Module: audit.ModuleProject, TargetType: "project",
			TargetID: projectRecID, TargetName: project.ProjectName,
			Summary: "FTP 远程推送 " + objectLabel + " → " + result.UpdatePath,
			Detail:  result.InfoContent,
			Success: true,
		})
		return &result, nil
	})
	httpx.WriteJSON(w, http.StatusAccepted, workspaceFtpPushStartResponse{
		TaskID: taskID,
		Status: "running",
	})
}

type resolvedFtpMergeSource struct {
	projectRecID string
	source       workspaceftppush.SourcePath
}

func normalizeRelPaths(paths []string, skip string) []string {
	skip = strings.TrimSpace(strings.ReplaceAll(skip, "\\", "/"))
	out := make([]string, 0, len(paths))
	seen := map[string]struct{}{}
	if skip != "" && skip != "." {
		seen[skip] = struct{}{}
	}
	for _, p := range paths {
		p = strings.TrimSpace(strings.ReplaceAll(p, "\\", "/"))
		if p == "" || p == "." {
			continue
		}
		if _, ok := seen[p]; ok {
			continue
		}
		seen[p] = struct{}{}
		out = append(out, p)
	}
	return out
}

func (h *Handler) resolveFtpMergeSources(anchor *repository.PackProject, srcs []workspaceFtpMergeSource) ([]resolvedFtpMergeSource, error) {
	if len(srcs) == 0 {
		return nil, nil
	}
	out := make([]resolvedFtpMergeSource, 0, len(srcs))
	for _, src := range srcs {
		id := strings.TrimSpace(src.ProjectRecID)
		rel := strings.TrimSpace(src.Path)
		if id == "" || rel == "" || rel == "." {
			return nil, errors.New("merge_sources require project_rec_id and path")
		}
		if id == anchor.RecID {
			continue
		}
		p, err := h.projects.FindByRecID(id)
		if err != nil {
			return nil, err
		}
		if p.WorkspaceRecID != anchor.WorkspaceRecID {
			return nil, errors.New("merge sources must belong to the same workspace")
		}
		_, _, base, err := h.resolveProjectDir(id)
		if err != nil {
			return nil, err
		}
		out = append(out, resolvedFtpMergeSource{
			projectRecID: id,
			source: workspaceftppush.SourcePath{
				LocalBase:    base,
				RelativePath: rel,
			},
		})
	}
	return out, nil
}
