package build

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"auto-pack/server/internal/access"
	"auto-pack/server/internal/batchlock"
	"auto-pack/server/internal/batchrun"
	"auto-pack/server/internal/build/record"
	"auto-pack/server/internal/iam"
	"auto-pack/server/internal/job"
	"auto-pack/server/internal/project"
	"auto-pack/server/internal/release"
	"auto-pack/server/internal/release/order"
	"auto-pack/server/internal/store"
	"auto-pack/server/internal/workspace"
)

// Service is the L3 workspace auto-build use-case layer.
type Service struct {
	st         *store.Store
	guard      *iam.Guard
	projects   *project.Service
	workspaces *workspace.Service
	jobs       *job.Service
	releases   *release.Service
	gate       *batchlock.Gate
	coord      *record.Coordinator
	dataDir    string
	wsRoot     string
	runCtx     context.Context
}

// NewService constructs a build Service. runCtx is the process lifetime used by the batch goroutine.
func NewService(runCtx context.Context, st *store.Store, guard *iam.Guard, projects *project.Service, workspaces *workspace.Service, jobs *job.Service, releases *release.Service, gate *batchlock.Gate, dataDir, wsRoot string) *Service {
	if runCtx == nil {
		runCtx = context.Background()
	}
	return &Service{
		st: st, guard: guard, projects: projects, workspaces: workspaces,
		jobs: jobs, releases: releases, gate: gate, coord: record.NewCoordinator(),
		dataDir: dataDir, wsRoot: wsRoot, runCtx: runCtx,
	}
}

// RecoverInterrupted marks leftover running rows failed after process start.
func (s *Service) RecoverInterrupted(ctx context.Context) error {
	return record.RecoverInterrupted(ctx, s.st)
}

// Start begins an async workspace auto-build.
func (s *Service) Start(ctx context.Context, actor iam.Actor, req StartRequest) (StartResponse, error) {
	if err := s.guard.Allow(ctx, actor, req.WorkspaceRecID, iam.ActionJobSubmit); err != nil {
		return StartResponse{}, err
	}
	body, err := normalizeStart(req)
	if err != nil {
		return StartResponse{}, err
	}
	ws, err := s.workspaces.GetRaw(ctx, body.WorkspaceRecID)
	if err != nil {
		return StartResponse{}, err
	}
	list, err := s.projects.ListRaw(ctx, body.WorkspaceRecID)
	if err != nil {
		return StartResponse{}, err
	}
	enabled := 0
	for _, p := range list {
		if p.IsEnabled != 0 {
			enabled++
		}
	}
	if enabled == 0 {
		return StartResponse{}, ErrNoProjects
	}
	if err := s.gate.Try(body.WorkspaceRecID, batchlock.KindBuild); err != nil {
		return StartResponse{}, err
	}
	started := false
	defer func() {
		if !started {
			s.gate.Done(body.WorkspaceRecID)
		}
	}()
	num, err := record.NextNum(ctx, s.st, body.WorkspaceRecID)
	if err != nil {
		return StartResponse{}, err
	}
	recID := access.NewID()
	logDir := filepath.Join(s.dataDir, "builds")
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		return StartResponse{}, err
	}
	logPath := filepath.Join(logDir, recID+".log")
	filter, _ := json.Marshal(map[string]any{
		"scope": body.Scope, "project_rec_ids": body.ProjectRecIDs, "include_non_maven": body.IncludeNonMaven,
		"exact_targets": body.ExactTargets,
	})
	rec := Record{
		RecID: recID, WorkspaceRecID: body.WorkspaceRecID, BuildNum: num, Status: RecordRunning,
		Scope: body.Scope, Message: "running", LogPath: logPath, FilterJSON: string(filter),
		StartedAt: time.Now().UTC().Format("2006-01-02 15:04:05"),
	}
	if err := record.Insert(ctx, s.st, rec); err != nil {
		return StartResponse{}, err
	}
	meta := record.Job{BuildRecID: recID, Scope: body.Scope}
	if err := s.coord.Start(s.runCtx, body.WorkspaceRecID, ws.WorkspaceName, enabled, meta, func(runCtx context.Context, job *record.Job) {
		defer s.gate.Done(body.WorkspaceRecID)
		s.runBatch(runCtx, actor, ws, list, body, job, logPath)
	}); err != nil {
		_ = record.Finish(context.Background(), s.st, recID, record.RecordFailed, err.Error(), 0, 0, 0)
		return StartResponse{}, err
	}
	started = true
	return StartResponse{
		Status: JobStatusRunning, BuildRecID: recID, WorkspaceRecID: body.WorkspaceRecID,
		WorkspaceName: ws.WorkspaceName, TotalCount: enabled,
	}, nil
}

func normalizeStart(req StartRequest) (StartRequest, error) {
	req.WorkspaceRecID = strings.TrimSpace(req.WorkspaceRecID)
	if req.WorkspaceRecID == "" {
		return req, fmt.Errorf("workspace_rec_id is required")
	}
	req.Scope = strings.TrimSpace(req.Scope)
	if req.Scope == "" {
		req.Scope = "workspace"
	}
	if req.Scope != "workspace" && req.Scope != "selected" {
		return req, fmt.Errorf("scope must be workspace or selected")
	}
	ids := make([]string, 0, len(req.ProjectRecIDs))
	seen := map[string]struct{}{}
	for _, id := range req.ProjectRecIDs {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	req.ProjectRecIDs = ids
	if req.Scope == "selected" && len(req.ProjectRecIDs) == 0 {
		return req, fmt.Errorf("project_rec_ids is required when scope is selected")
	}
	return req, nil
}

func (s *Service) runBatch(ctx context.Context, actor iam.Actor, ws workspace.Workspace, list []project.Project, body StartRequest, job *record.Job, logPath string) {
	s.appendLog(logPath, stamp()+" 开始自动构建")
	analysis := order.AnalyzeMavenWorkspace(list, func(prj project.Project) string {
		return s.workDir(ws, prj)
	})
	for _, w := range analysis.Warnings {
		s.appendLog(logPath, stamp()+" "+w)
	}
	targets := body.ProjectRecIDs
	if body.Scope != "selected" {
		targets = nil
	}
	queue := order.Queue(list, analysis, order.QueueOpts{
		TargetRecIDs:  targets,
		IncludeOthers: body.Scope == "workspace" && body.IncludeNonMaven,
		ExactTargets:  body.Scope == "selected" && body.ExactTargets,
	})
	if len(queue) == 0 {
		s.appendLog(logPath, stamp()+" 没有可构建的工程")
		_ = record.Finish(context.Background(), s.st, job.BuildRecID, record.RecordFailed, "no projects", 0, 0, 0)
		return
	}
	s.coord.SetProgress(job, "", 0, len(queue))
	s.appendLog(logPath, stamp()+fmt.Sprintf(" 构建队列 %d 个工程", len(queue)))
	for i, p := range queue {
		s.appendLog(logPath, fmt.Sprintf("  %d. %s", i+1, p.ProjectName))
	}
	s.coord.SetPhase(job, "sync")
	var sync release.SyncResult
	if s.releases != nil {
		ids := make([]string, 0, len(queue))
		for _, p := range queue {
			ids = append(ids, p.RecID)
		}
		s.appendLog(logPath, stamp()+fmt.Sprintf(" 预同步本次队列 %d 个工程", len(ids)))
		var err error
		sync, err = s.releases.SyncSourcesLimited(ctx, actor, body.WorkspaceRecID, ids, func(ev release.SyncEvent) {
			if line := formatSyncEvent(ev); line != "" {
				s.appendLog(logPath, line)
			}
		})
		if err != nil && ctx.Err() == nil {
			s.appendLog(logPath, stamp()+" 预同步失败："+err.Error())
		}
	}
	s.coord.SetPhase(job, "build")
	succeeded, skipped, failed := 0, 0, 0
	outcomes := map[string]string{}
	for i, prj := range queue {
		if err := ctx.Err(); err != nil {
			failed++
			res := record.Result{ProjectName: prj.ProjectName, Status: "failed", Message: err.Error()}
			s.coord.AppendResult(job, res, succeeded, skipped, failed)
			s.appendLog(logPath, stamp()+" 已取消")
			break
		}
		s.coord.SetProgress(job, prj.ProjectName, i, len(queue))
		s.appendLog(logPath, stamp()+fmt.Sprintf(" 正在构建（%d/%d）：%s", i+1, len(queue), prj.ProjectName))
		out := batchrun.RunBuild(ctx, s.jobs, actor, body.WorkspaceRecID, prj.RecID, sync.SyncedReady(prj.ProjectName), func(step string) {
			s.appendLog(logPath, stamp()+fmt.Sprintf(" %s · %s", prj.ProjectName, step))
		})
		res := record.Result{ProjectName: prj.ProjectName, Message: out.Message}
		if out.OK {
			succeeded++
			res.Status = "ok"
			outcomes[prj.RecID] = "ok"
			s.appendLog(logPath, stamp()+fmt.Sprintf(" 构建成功：%s", prj.ProjectName))
		} else {
			failed++
			res.Status = "failed"
			outcomes[prj.RecID] = "failed"
			s.appendLog(logPath, stamp()+fmt.Sprintf(" 构建失败：%s — %s", prj.ProjectName, out.Message))
		}
		s.coord.AppendResult(job, res, succeeded, skipped, failed)
	}
	cycles := order.CyclicRecIDs(analysis)
	if failed > 0 && len(cycles) > 0 {
		s.coord.SetPhase(job, "retry")
		s.appendLog(logPath, stamp()+" 循环依赖工程重试一轮")
		for _, prj := range queue {
			if _, ok := cycles[prj.RecID]; !ok || outcomes[prj.RecID] != "failed" {
				continue
			}
			s.appendLog(logPath, stamp()+" 重试构建："+prj.ProjectName)
			out := batchrun.RunBuild(ctx, s.jobs, actor, body.WorkspaceRecID, prj.RecID, sync.SyncedReady(prj.ProjectName), nil)
			if out.OK {
				failed--
				succeeded++
				outcomes[prj.RecID] = "ok"
				s.appendLog(logPath, stamp()+" 重试成功："+prj.ProjectName)
			} else {
				s.appendLog(logPath, stamp()+" 重试失败："+prj.ProjectName+" — "+out.Message)
			}
		}
	}
	s.coord.FinishResult(job, succeeded, skipped, failed)
	status := record.ResolveStatus(succeeded, failed)
	summary := fmt.Sprintf("succeeded %d, skipped %d, failed %d", succeeded, skipped, failed)
	s.appendLog(logPath, stamp()+" 自动构建结束："+fmt.Sprintf("成功 %d，跳过 %d，失败 %d", succeeded, skipped, failed))
	if err := record.Finish(context.Background(), s.st, job.BuildRecID, status, summary, succeeded, skipped, failed); err != nil {
		log.Printf("build finish %s: %v", job.BuildRecID, err)
	}
}

// Status returns the coordinator snapshot, falling back to a running DB row.
func (s *Service) Status(ctx context.Context, actor iam.Actor, workspaceID string) (Status, error) {
	if err := s.guard.CanRead(ctx, actor, workspaceID); err != nil {
		return Status{}, err
	}
	st := s.coord.Status(workspaceID)
	if st.Status == JobStatusRunning {
		if rec, err := record.FindRunning(ctx, s.st, workspaceID); err == nil && rec.RecID != "" && st.BuildRecID == "" {
			st.BuildRecID = rec.RecID
		}
		return st, nil
	}
	if rec, err := record.FindRunning(ctx, s.st, workspaceID); err == nil && rec.RecID != "" {
		return Status{Job: record.Job{
			BuildRecID: rec.RecID, WorkspaceRecID: rec.WorkspaceRecID, Status: JobStatusRunning,
			Scope: rec.Scope, Succeeded: rec.Succeeded, Skipped: rec.Skipped, Failed: rec.Failed,
		}}, nil
	}
	return st, nil
}

// History lists persisted workspace builds.
func (s *Service) History(ctx context.Context, actor iam.Actor, workspaceID string, limit int) ([]Record, error) {
	if err := s.guard.CanRead(ctx, actor, workspaceID); err != nil {
		return nil, err
	}
	return record.List(ctx, s.st, workspaceID, limit)
}

// Log tails a persisted build log.
func (s *Service) Log(ctx context.Context, actor iam.Actor, workspaceID, buildID string, offset int64) (LogView, error) {
	if err := s.guard.CanRead(ctx, actor, workspaceID); err != nil {
		return LogView{}, err
	}
	rec, err := record.Get(ctx, s.st, buildID)
	if err != nil {
		return LogView{}, err
	}
	if rec.WorkspaceRecID != workspaceID {
		return LogView{}, ErrNotFound
	}
	text, size, used, err := record.ReadLogTail(rec.LogPath, offset)
	if err != nil {
		return LogView{}, err
	}
	return LogView{Content: text, Offset: used, Size: size, Complete: rec.Status != RecordRunning, Status: rec.Status, Build: rec}, nil
}

func (s *Service) workDir(ws workspace.Workspace, prj project.Project) string {
	recipe := project.ParseRecipe(prj.RecipeJSON)
	if abs := project.AbsLocalSource(recipe); abs != "" {
		return abs
	}
	rel := strings.TrimSpace(prj.WorkRelPath)
	if rel == "" {
		rel = project.WorkRel(prj.ProjectName, recipe)
	}
	return workspace.ProjectDir(s.wsRoot, ws.WorkspaceCode, ws.RecID, rel)
}

func (s *Service) appendLog(path, line string) {
	if err := record.AppendLog(path, line); err != nil {
		log.Printf("build log %s: %v", path, err)
	}
}

func stamp() string {
	return "[" + time.Now().Format("2006-01-02 15:04:05") + "]"
}

func formatSyncEvent(ev release.SyncEvent) string {
	switch ev.Event {
	case "start":
		return stamp() + fmt.Sprintf(" 预同步开始，共 %d 个工程", ev.Total)
	case "begin":
		if ev.Item == nil {
			return ""
		}
		return stamp() + fmt.Sprintf(" 预同步（%d/%d）：%s %s", ev.Index, ev.Total, ev.Item.ProjectName, ev.Item.Action)
	case "item":
		if ev.Item == nil {
			return ""
		}
		line := stamp() + fmt.Sprintf(" [%s] %s", ev.Item.Outcome, ev.Item.ProjectName)
		if ev.Item.Message != "" {
			line += " " + ev.Item.Message
		}
		return line
	case "done":
		return stamp() + fmt.Sprintf(" 预同步结束：成功 %d，跳过 %d，失败 %d", ev.Synced, ev.Skipped, ev.Failed)
	case "error":
		if ev.Message == "" {
			return ""
		}
		return stamp() + " 预同步错误：" + ev.Message
	default:
		return ""
	}
}
