package release

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"auto-pack/server/internal/access"
	"auto-pack/server/internal/batchlock"
	"auto-pack/server/internal/env"
	"auto-pack/server/internal/host/git"
	"auto-pack/server/internal/iam"
	"auto-pack/server/internal/job"
	"auto-pack/server/internal/project"
	"auto-pack/server/internal/release/notes"
	"auto-pack/server/internal/release/order"
	"auto-pack/server/internal/release/publish"
	"auto-pack/server/internal/release/record"
	"auto-pack/server/internal/release/suggest"
	"auto-pack/server/internal/store"
	"auto-pack/server/internal/workspace"
)

// Service is the L3 workspace auto-release use-case layer.
type Service struct {
	st         *store.Store
	guard      *iam.Guard
	projects   *project.Service
	workspaces *workspace.Service
	envs       *env.Service
	jobs       *job.Service
	gate       *batchlock.Gate
	coord      *record.Coordinator
	dataDir    string
	wsRoot     string
	depth      int
	runCtx     context.Context
	syncMu     sync.Mutex
	syncing    map[string]*syncBroadcast
}

// NewService constructs a release Service. runCtx is the process lifetime used by the batch goroutine.
func NewService(runCtx context.Context, st *store.Store, guard *iam.Guard, projects *project.Service, workspaces *workspace.Service, envs *env.Service, jobs *job.Service, gate *batchlock.Gate, dataDir, wsRoot string, depth int) *Service {
	if runCtx == nil {
		runCtx = context.Background()
	}
	return &Service{
		st: st, guard: guard, projects: projects, workspaces: workspaces, envs: envs, jobs: jobs, gate: gate,
		coord: record.NewCoordinator(), dataDir: dataDir, wsRoot: wsRoot, depth: depth, runCtx: runCtx,
		syncing: map[string]*syncBroadcast{},
	}
}

// RecoverInterrupted marks leftover running rows failed after process start.
func (s *Service) RecoverInterrupted(ctx context.Context) error {
	return record.RecoverInterrupted(ctx, s.st)
}

// Start begins an async workspace auto-release.
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
	if len(list) == 0 {
		return StartResponse{}, ErrNoProjects
	}
	if err := s.gate.Try(body.WorkspaceRecID, batchlock.KindRelease); err != nil {
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
	logDir := filepath.Join(s.dataDir, "releases")
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		return StartResponse{}, err
	}
	logPath := filepath.Join(logDir, recID+".log")
	target := ""
	if body.CreateTargetBranch != nil && *body.CreateTargetBranch {
		target = strings.TrimSpace(body.TargetBranch)
	}
	postJSON, err := MarshalPostOptions(body.PostOptions)
	if err != nil {
		return StartResponse{}, err
	}
	rec := Record{
		RecID: recID, WorkspaceRecID: body.WorkspaceRecID, ReleaseNum: num, Status: RecordRunning,
		ReleaseBranch: body.ReleaseBranch, TagBranch: body.TagBranch, TargetBranch: target,
		Message: "running", LogPath: logPath, PostOptionsJSON: postJSON,
		StartedAt: time.Now().UTC().Format("2006-01-02 15:04:05"),
	}
	if err := record.Insert(ctx, s.st, rec); err != nil {
		return StartResponse{}, err
	}
	meta := record.Job{ReleaseRecID: recID, Tag: body.TagBranch, ReleaseBranch: body.ReleaseBranch, PostOptions: body.PostOptions}
	if err := s.coord.Start(s.runCtx, body.WorkspaceRecID, ws.WorkspaceName, len(list), meta, func(runCtx context.Context, job *record.Job) {
		defer s.gate.Done(body.WorkspaceRecID)
		s.runBatch(runCtx, actor, ws, list, body, job, logPath)
	}); err != nil {
		_ = record.Finish(context.Background(), s.st, recID, RecordFailed, err.Error(), 0, 0, 0)
		if errors.Is(err, ErrReleaseInProgress) {
			return StartResponse{}, err
		}
		return StartResponse{}, err
	}
	started = true
	return StartResponse{
		Status: JobStatusRunning, ReleaseRecID: recID, WorkspaceRecID: body.WorkspaceRecID,
		WorkspaceName: ws.WorkspaceName, ReleaseBranch: body.ReleaseBranch, Tag: body.TagBranch,
	}, nil
}

func normalizeStart(req StartRequest) (StartRequest, error) {
	req.WorkspaceRecID = strings.TrimSpace(req.WorkspaceRecID)
	if req.WorkspaceRecID == "" {
		return req, errors.New("workspace_rec_id is required")
	}
	req.ReleaseBranch = strings.TrimSpace(req.ReleaseBranch)
	if req.ReleaseBranch == "" {
		return req, errors.New("release_branch is required")
	}
	if err := project.CheckRepoBranch(req.ReleaseBranch); err != nil {
		return req, fmt.Errorf("release_branch: %w", err)
	}
	req.TagBranch = suggest.NormalizeVersionTag(req.TagBranch)
	if req.TagBranch == "" {
		return req, errors.New("tag_branch is required")
	}
	if err := suggest.ValidateVersionTag(req.TagBranch); err != nil {
		return req, err
	}
	req.ReleaseDate = strings.TrimSpace(req.ReleaseDate)
	if req.ReleaseDate == "" {
		req.ReleaseDate = time.Now().Format("2006-01-02")
	}
	req.TargetBranch = strings.TrimSpace(req.TargetBranch)
	createTarget := false
	if req.CreateTargetBranch != nil {
		createTarget = *req.CreateTargetBranch
	}
	if createTarget {
		if req.TargetBranch == "" {
			return req, errors.New("target_branch is required when create_target_branch is enabled")
		}
		if err := project.CheckRepoBranch(req.TargetBranch); err != nil {
			return req, fmt.Errorf("target_branch: %w", err)
		}
	}
	return req, nil
}

func (s *Service) runBatch(ctx context.Context, actor iam.Actor, ws workspace.Workspace, list []project.Project, body StartRequest, job *record.Job, logPath string) {
	allowRepeat := true
	if body.AllowRepeat != nil {
		allowRepeat = *body.AllowRepeat
	}
	updatePom := true
	if body.UpdatePomVersion != nil {
		updatePom = *body.UpdatePomVersion
	}
	createTarget := false
	if body.CreateTargetBranch != nil {
		createTarget = *body.CreateTargetBranch
	}
	releaser := &Releaser{Git: &publish.GitCommitter{OptFor: func(prj project.Project) git.Options {
		return s.gitOpt(ctx, prj)
	}}}
	var results []Result
	released, skipped, failed := 0, 0, 0
	for _, prj := range list {
		if err := ctx.Err(); err != nil {
			out := Result{ProjectName: prj.ProjectName, Status: record.ResultFailed, Message: err.Error()}
			failed++
			results = append(results, out)
			s.coord.AppendResult(job, out, released, skipped, failed)
			break
		}
		s.coord.SetProgress(job, prj.ProjectName, len(results))
		s.writeLog(logPath, record.FormatBatchReleaseLog(s.logParams(body, results, prj.ProjectName, false, released, skipped, failed)))
		work := s.workDir(ws, prj)
		out := releaser.Run(ctx, prj, Request{
			WorkDir: work, ReleaseBranch: body.ReleaseBranch, Tag: body.TagBranch,
			ReleaseNote: body.ReleaseNote, ReleaseDate: body.ReleaseDate,
			AllowRepeat: allowRepeat, UpdatePomVersion: updatePom,
			CreateTargetBranch: createTarget, TargetBranch: body.TargetBranch,
		})
		switch out.Status {
		case record.ResultOK:
			released++
		case record.ResultSkipped:
			skipped++
		default:
			failed++
		}
		results = append(results, out)
		s.coord.AppendResult(job, out, released, skipped, failed)
	}
	s.coord.FinishResult(job, released, skipped, failed, results)
	s.writeLog(logPath, record.FormatBatchReleaseLog(s.logParams(body, results, "", true, released, skipped, failed)))
	s.runPost(ctx, actor, ws, list, body, job, logPath, results, released)
	status := record.ResolveStatus(released, failed)
	summary := fmt.Sprintf("released %d, skipped %d, failed %d", released, skipped, failed)
	if err := record.Finish(context.Background(), s.st, job.ReleaseRecID, status, summary, released, skipped, failed); err != nil {
		log.Printf("release finish %s: %v", job.ReleaseRecID, err)
	}
	if released > 0 {
		branch := suggest.RepoBranchAfterRelease(body.ReleaseBranch, body.TargetBranch, createTarget)
		for _, prj := range list {
			if project.ParseRecipe(prj.RecipeJSON).IsLocal() {
				continue
			}
			if err := s.projects.SetRepoBranch(context.Background(), prj.RecID, branch); err != nil {
				log.Printf("release set repo_branch %s -> %s: %v", prj.ProjectName, branch, err)
			}
		}
	}
}

func (s *Service) logParams(body StartRequest, results []Result, current string, final bool, released, skipped, failed int) record.BatchLogParams {
	return record.BatchLogParams{
		ReleaseBranch: body.ReleaseBranch, Tag: body.TagBranch, TargetBranch: body.TargetBranch,
		ReleaseNote: notes.ResolveCommitMessage(body.ReleaseNote, body.TagBranch), ReleaseDate: body.ReleaseDate,
		StartLabel: "开始自动发版", DoneLabel: "自动发版结束", ProgressLabel: "正在发版",
		ParamsLine:   fmt.Sprintf("发版分支 %s，版本 Tag %s，目标分支 %s", body.ReleaseBranch, body.TagBranch, orDash(body.TargetBranch)),
		SummaryLine:  fmt.Sprintf("成功 %d，跳过 %d，失败 %d，Tag %s", released, skipped, failed, body.TagBranch),
		NoStepsLabel: "无步骤", NoMessageLabel: "无错误信息", InProgressLabel: "正在处理：%s",
		InProgressProject: current, Final: final, Running: !final, Results: results,
		Released: released, Skipped: skipped, Failed: failed,
	}
}

// Status returns the coordinator snapshot, falling back to a running DB row.
func (s *Service) Status(ctx context.Context, actor iam.Actor, workspaceID string) (Status, error) {
	if err := s.guard.CanRead(ctx, actor, workspaceID); err != nil {
		return Status{}, err
	}
	st := s.coord.Status(workspaceID)
	if st.Status == JobStatusRunning {
		if rec, err := record.FindRunning(ctx, s.st, workspaceID); err == nil && rec.RecID != "" {
			st.PostOptions, _ = ParsePostOptionsJSON(rec.PostOptionsJSON)
			if st.ReleaseRecID == "" {
				st.ReleaseRecID = rec.RecID
			}
		}
		return st, nil
	}
	if rec, err := record.FindRunning(ctx, s.st, workspaceID); err == nil && rec.RecID != "" {
		opts, _ := ParsePostOptionsJSON(rec.PostOptionsJSON)
		return Status{Job: record.Job{
			ReleaseRecID: rec.RecID, WorkspaceRecID: rec.WorkspaceRecID, Status: JobStatusRunning,
			Tag: rec.TagBranch, ReleaseBranch: rec.ReleaseBranch, Released: rec.Released,
			Skipped: rec.Skipped, Failed: rec.Failed, PostOptions: opts,
		}}, nil
	}
	return st, nil
}

// History lists persisted workspace releases.
func (s *Service) History(ctx context.Context, actor iam.Actor, workspaceID string, limit int) ([]Record, error) {
	if err := s.guard.CanRead(ctx, actor, workspaceID); err != nil {
		return nil, err
	}
	return record.List(ctx, s.st, workspaceID, limit)
}

// Log tails a persisted release log.
func (s *Service) Log(ctx context.Context, actor iam.Actor, workspaceID, releaseID string, offset int64) (LogView, error) {
	if err := s.guard.CanRead(ctx, actor, workspaceID); err != nil {
		return LogView{}, err
	}
	rec, err := record.Get(ctx, s.st, releaseID)
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
	return LogView{Content: text, Offset: used, Size: size, Complete: rec.Status != RecordRunning, Status: rec.Status, Release: rec}, nil
}

// SavePostOptions updates post options on the running release.
func (s *Service) SavePostOptions(ctx context.Context, actor iam.Actor, workspaceID string, opts PostOptions) (string, error) {
	if err := s.guard.Allow(ctx, actor, workspaceID, iam.ActionJobSubmit); err != nil {
		return "", err
	}
	st := s.coord.Status(workspaceID)
	recID := st.ReleaseRecID
	if recID == "" {
		rec, err := record.FindRunning(ctx, s.st, workspaceID)
		if err != nil {
			return "", err
		}
		recID = rec.RecID
	}
	if recID == "" {
		return "", ErrNoActive
	}
	raw, err := MarshalPostOptions(&opts)
	if err != nil {
		return "", err
	}
	if err := record.UpdatePostOptions(ctx, s.st, recID, raw); err != nil {
		return "", err
	}
	s.coord.SetPostOptions(workspaceID, &opts)
	return recID, nil
}

// Defaults suggests release branch and next tag.
func (s *Service) Defaults(ctx context.Context, actor iam.Actor, workspaceID string) (DefaultsResponse, error) {
	if err := s.guard.CanRead(ctx, actor, workspaceID); err != nil {
		return DefaultsResponse{}, err
	}
	ws, err := s.workspaces.GetRaw(ctx, workspaceID)
	if err != nil {
		return DefaultsResponse{}, err
	}
	code := strings.TrimSpace(ws.WorkspaceCode)
	out := DefaultsResponse{ReleaseBranch: suggest.DefaultReleaseBranch(code), TagBranch: code, TagSource: "workspace"}
	list, err := s.projects.ListRaw(ctx, workspaceID)
	if err != nil {
		return out, nil
	}
	var tags []string
	for _, prj := range list {
		if strings.TrimSpace(prj.RepoURL) == "" || project.ParseRecipe(prj.RecipeJSON).IsLocal() {
			continue
		}
		work := s.workDir(ws, prj)
		if !git.IsRepo(work) {
			continue
		}
		tags = append(tags, publish.ListRemoteTags(ctx, work, s.gitOpt(ctx, prj))...)
		if len(tags) > 0 {
			out.TagSource = "remote"
			break
		}
	}
	if len(tags) == 0 {
		if hist, err := record.ListTagHistory(ctx, s.st, workspaceID, 50); err == nil && len(hist) > 0 {
			tags = hist
			out.TagSource = "history"
		}
	}
	if next := suggest.SuggestNextVersionTag(code, tags); next != "" {
		out.TagBranch = next
	}
	return out, nil
}

// SyncSources fetches Git projects in a workspace sequentially.
func (s *Service) SyncSources(ctx context.Context, actor iam.Actor, workspaceID string) (SyncResult, error) {
	return s.SyncSourcesProgress(ctx, actor, workspaceID, nil)
}

// SyncSourcesProgress 与 SyncSources 相同，并在每个工程开始/结束时回调，供 HTTP 流式日志。
// 同一空间已有预同步时不报冲突，而是回放已有事件并跟到结束。
func (s *Service) SyncSourcesProgress(ctx context.Context, actor iam.Actor, workspaceID string, progress SyncListener) (SyncResult, error) {
	if err := s.guard.Allow(ctx, actor, workspaceID, iam.ActionJobSubmit); err != nil {
		return SyncResult{}, err
	}
	run, err := s.acquireSync(workspaceID)
	if errors.Is(err, ErrSyncInProgress) {
		return run.follow(ctx, progress)
	}
	if err != nil {
		return SyncResult{}, err
	}
	defer s.endSync(workspaceID)
	var out SyncResult
	var runErr error
	defer func() { run.finish(out, runErr) }()
	ws, err := s.workspaces.GetRaw(ctx, workspaceID)
	if err != nil {
		runErr = err
		return SyncResult{}, err
	}
	list, err := s.projects.ListRaw(ctx, workspaceID)
	if err != nil {
		runErr = err
		return SyncResult{}, err
	}
	emit := func(ev SyncEvent) {
		run.emit(ev)
		if progress != nil {
			progress(ev)
		}
	}
	total := len(list)
	emit(SyncEvent{Event: "start", Total: total})
	for i, prj := range list {
		item := SyncItem{ProjectName: prj.ProjectName, Branch: strings.TrimSpace(prj.RepoBranch)}
		recipe := project.ParseRecipe(prj.RecipeJSON)
		finish := func() {
			out.Items = append(out.Items, item)
			copied := item
			emit(SyncEvent{Event: "item", Index: i + 1, Total: total, Item: &copied, Synced: out.Synced, Skipped: out.Skipped, Failed: out.Failed})
		}
		if prj.IsEnabled == 0 {
			out.Skipped++
			item.Outcome = "skipped"
			item.Message = "disabled"
			finish()
			continue
		}
		if recipe.IsLocal() {
			out.Skipped++
			item.Outcome = "skipped"
			item.Message = "local"
			finish()
			continue
		}
		if strings.TrimSpace(prj.RepoURL) == "" {
			out.Skipped++
			item.Outcome = "skipped"
			item.Message = "no_repo"
			finish()
			continue
		}
		work := s.workDir(ws, prj)
		opt := s.gitOpt(ctx, prj)
		if git.IsRepo(work) {
			item.Action = "fetch"
		} else {
			item.Action = "clone"
		}
		begin := item
		emit(SyncEvent{Event: "begin", Index: i + 1, Total: total, Item: &begin})
		var buf bytes.Buffer
		var syncErr error
		if item.Action == "fetch" {
			syncErr = git.FetchPull(ctx, work, item.Branch, opt, &buf)
		} else {
			syncErr = git.Clone(ctx, prj.RepoURL, work, item.Branch, opt, &buf)
		}
		item.Log = clipSyncLog(buf.String())
		if syncErr != nil {
			out.Failed++
			item.Outcome = "failed"
			item.Message = syncErr.Error()
			out.FailedItems = append(out.FailedItems, prj.ProjectName+": "+syncErr.Error())
			finish()
			continue
		}
		if tip := git.ShortHead(ctx, work, opt); tip != "" {
			item.Message = tip
		}
		out.Synced++
		item.Outcome = "synced"
		finish()
	}
	emit(SyncEvent{Event: "done", Total: total, Synced: out.Synced, Skipped: out.Skipped, Failed: out.Failed})
	return out, nil
}

func clipSyncLog(raw string) string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return ""
	}
	const max = 4000
	if len(s) <= max {
		return s
	}
	return s[:max] + "\n…"
}

// MavenOrder returns workspace Maven / npm dependency order for auto-build.
func (s *Service) MavenOrder(ctx context.Context, actor iam.Actor, workspaceID string) (MavenAnalysis, error) {
	if err := s.guard.CanRead(ctx, actor, workspaceID); err != nil {
		return MavenAnalysis{}, err
	}
	ws, err := s.workspaces.GetRaw(ctx, workspaceID)
	if err != nil {
		return MavenAnalysis{}, err
	}
	list, err := s.projects.ListRaw(ctx, workspaceID)
	if err != nil {
		return MavenAnalysis{}, err
	}
	return order.AnalyzeMavenWorkspace(list, func(prj project.Project) string {
		return s.workDir(ws, prj)
	}), nil
}

func (s *Service) gitOpt(ctx context.Context, prj project.Project) git.Options {
	recipe := project.ParseRecipe(prj.RecipeJSON)
	cfg := s.envCfg(ctx, prj.WorkspaceRecID, recipe.GitEnvRecID, "git")
	return git.Options{Depth: s.depth, Auth: git.MapAuth(cfg)}
}

func (s *Service) envCfg(ctx context.Context, workspaceID, recID, kind string) map[string]any {
	if s.envs == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if strings.TrimSpace(recID) != "" {
		if _, cfg, err := s.envs.DecryptRow(ctx, recID); err == nil {
			return cfg
		}
	}
	return s.envs.KindConfig(ctx, workspaceID, kind)
}

func (s *Service) workDir(ws workspace.Workspace, prj project.Project) string {
	rel := strings.TrimSpace(prj.WorkRelPath)
	if rel == "" {
		rel = project.WorkRel(prj.ProjectName, project.ParseRecipe(prj.RecipeJSON))
	}
	return workspace.ProjectDir(s.wsRoot, ws.WorkspaceCode, ws.RecID, rel)
}

func (s *Service) writeLog(path, content string) {
	if err := record.WriteLog(path, content); err != nil {
		log.Printf("release log %s: %v", path, err)
	}
}

func (s *Service) beginSync(workspaceID string) error {
	_, err := s.acquireSync(workspaceID)
	return err
}

func (s *Service) acquireSync(workspaceID string) (*syncBroadcast, error) {
	id := strings.TrimSpace(workspaceID)
	s.syncMu.Lock()
	defer s.syncMu.Unlock()
	if s.syncing == nil {
		s.syncing = map[string]*syncBroadcast{}
	}
	if run, ok := s.syncing[id]; ok {
		return run, ErrSyncInProgress
	}
	run := newSyncBroadcast()
	s.syncing[id] = run
	return run, nil
}

func (s *Service) endSync(workspaceID string) {
	s.syncMu.Lock()
	defer s.syncMu.Unlock()
	delete(s.syncing, strings.TrimSpace(workspaceID))
}

func orDash(s string) string {
	if strings.TrimSpace(s) == "" {
		return "—"
	}
	return s
}
