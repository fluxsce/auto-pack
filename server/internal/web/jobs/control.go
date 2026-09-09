package jobs

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"auto-pack/server/internal/audit"
	"auto-pack/server/internal/job"
	"auto-pack/server/internal/project"
	"auto-pack/server/internal/web/httpx"
	"auto-pack/server/internal/web/middleware"
)

// Controller 处理作业 HTTP，只做参数与状态码。
type Controller struct {
	jobs     *job.Service
	projects *project.Service
	audit    *audit.Service
}

// New 构造作业控制器。
func New(jobs *job.Service, projects *project.Service, audit *audit.Service) *Controller {
	return &Controller{jobs: jobs, projects: projects, audit: audit}
}

// List 分页列出空间内作业。
func (c *Controller) List(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	page, _ := strconv.Atoi(q.Get("page"))
	pageSize, _ := strconv.Atoi(q.Get("page_size"))
	list, err := c.jobs.List(r.Context(), middleware.ActorFrom(r), job.ListQuery{
		WorkspaceID: q.Get("workspace_rec_id"),
		ProjectID:   q.Get("project_rec_id"),
		JobType:     q.Get("job_type"),
		Page:        page,
		PageSize:    pageSize,
	})
	if httpx.WriteSvcErr(w, err) {
		return
	}
	httpx.WriteJSON(w, http.StatusOK, list)
}

// Submit 入队作业。
func (c *Controller) Submit(w http.ResponseWriter, r *http.Request) {
	var req job.SubmitRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid body")
		return
	}
	actor := middleware.ActorFrom(r)
	j, err := c.jobs.Submit(r.Context(), actor, req)
	c.audit.Note(r.Context(), actor, req.WorkspaceRecID, audit.ActionJobSubmit, "job", j.RecID, "submit "+req.JobType, err)
	if httpx.WriteSvcErr(w, err) {
		return
	}
	httpx.WriteJSON(w, http.StatusOK, j)
}

// SubmitBatch 按空间内匹配工程批量入队同一类作业。
func (c *Controller) SubmitBatch(w http.ResponseWriter, r *http.Request) {
	var req job.BatchSubmitRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid body")
		return
	}
	actor := middleware.ActorFrom(r)
	if err := validateBatchPayload(req.JobType, req.Payload); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	list, err := c.projects.List(r.Context(), actor, req.WorkspaceRecID)
	if httpx.WriteSvcErr(w, err) {
		return
	}
	out := job.BatchSubmitResult{}
	for _, p := range list {
		if !matchBatchProject(req.JobType, p) {
			out.Skipped++
			continue
		}
		j, err := c.jobs.Submit(r.Context(), actor, job.SubmitRequest{
			WorkspaceRecID: req.WorkspaceRecID,
			ProjectRecID:   p.RecID,
			JobType:        req.JobType,
			Payload:        req.Payload,
		})
		if httpx.WriteSvcErr(w, err) {
			return
		}
		out.Queued++
		out.JobIDs = append(out.JobIDs, j.RecID)
	}
	c.audit.Note(r.Context(), actor, req.WorkspaceRecID, audit.ActionJobSubmit, "workspace", req.WorkspaceRecID, "batch "+req.JobType, nil)
	httpx.WriteJSON(w, http.StatusOK, out)
}

func validateBatchPayload(jobType string, p job.Payload) error {
	switch jobType {
	case job.TypeBranchClone:
		if err := project.CheckRepoBranch(p.Branch); err != nil {
			return err
		}
		if strings.TrimSpace(p.FromBranch) != "" {
			return project.CheckRepoBranch(p.FromBranch)
		}
		return nil
	case job.TypePOMVersion:
		if strings.TrimSpace(p.POMVersion) == "" && strings.TrimSpace(p.Tag) == "" {
			return job.ErrPOMVersion
		}
		return nil
	default:
		return job.ErrUnknownBatchType
	}
}

func matchBatchProject(jobType string, p project.Project) bool {
	recipe := project.ParseRecipe(p.RecipeJSON)
	switch jobType {
	case job.TypeBranchClone:
		return !recipe.IsLocal()
	case job.TypePOMVersion:
		return strings.EqualFold(strings.TrimSpace(p.ProjectType), "maven")
	default:
		return false
	}
}

// Get 返回作业与步骤。
func (c *Controller) Get(w http.ResponseWriter, r *http.Request) {
	j, steps, err := c.jobs.Get(r.Context(), middleware.ActorFrom(r), r.PathValue("id"))
	if httpx.WriteSvcErr(w, err) {
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"job": j, "steps": steps})
}

// Log 返回作业日志正文。offset 为字节偏移，只回这段之后的新增内容。
func (c *Controller) Log(w http.ResponseWriter, r *http.Request) {
	offset, _ := strconv.ParseInt(r.URL.Query().Get("offset"), 10, 64)
	text, size, used, err := c.jobs.Log(r.Context(), middleware.ActorFrom(r), r.PathValue("id"), offset)
	if httpx.WriteSvcErr(w, err) {
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"log": text, "size": size, "offset": used})
}

// Cancel 取消排队或运行中的作业。
func (c *Controller) Cancel(w http.ResponseWriter, r *http.Request) {
	actor := middleware.ActorFrom(r)
	id := r.PathValue("id")
	j, _, _ := c.jobs.Get(r.Context(), actor, id)
	err := c.jobs.Cancel(r.Context(), actor, id)
	c.audit.Note(r.Context(), actor, j.WorkspaceRecID, audit.ActionJobCancel, "job", id, "cancel job", err)
	if httpx.WriteSvcErr(w, err) {
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
