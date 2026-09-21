// Package runner 跑已入队作业。无 HTTP、无 iam。
//
// 按步骤分文件，避免单文件继续膨胀：
//   - engine.go  队列、项目锁、步骤分发
//   - sync.go    源码同步与 Git Tag
//   - build.go   构建、Version.txt、POM
//   - post.go    构建后操作与 FTP/SFTP 上传
//   - finish.go  成功后的依赖入队、邮件
//   - prune.go   后台按保留天数清理日志与库记录
//   - support.go 检出目录、环境解析、作业日志
package runner

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"auto-pack/server/internal/env"
	"auto-pack/server/internal/host/git"
	"auto-pack/server/internal/job"
	"auto-pack/server/internal/project"
	"auto-pack/server/internal/settings"
	"auto-pack/server/internal/store"
)

// Engine 按系统并发数执行队列中的作业；同一工程仍串行。
// 打包队列与 FTP/SFTP 推送队列分开，推送不占打包并发。
type Engine struct {
	st       *store.Store
	projects *project.Service
	envs     *env.Service
	dataDir  string
	wsRoot   string
	depth    int

	queue     chan string
	pushQueue chan string
	mu        sync.Mutex
	pruneMu   sync.Mutex
	locks     map[string]*sync.Mutex
	runCtx    context.Context
	stop      context.CancelFunc
	submit    SubmitFunc
	settings  *settings.Service
}

// SubmitFunc enqueues a follow-up job (dependents). Injected after job.Service exists.
type SubmitFunc func(ctx context.Context, req job.SubmitRequest) (job.Job, error)

// SetSubmit wires the job control-plane enqueue used for dependent projects.
func (e *Engine) SetSubmit(fn SubmitFunc) {
	e.submit = fn
}

// SetSettings wires platform props (concurrency, log retention) and global vars.
func (e *Engine) SetSettings(s *settings.Service) {
	e.settings = s
}

// NewEngine constructs an Engine. Call Start after migrate.
func NewEngine(st *store.Store, projects *project.Service, envs *env.Service, dataDir, wsRoot string, cloneDepth int) *Engine {
	if strings.TrimSpace(wsRoot) == "" {
		wsRoot = filepath.Join(dataDir, "workspaces")
	}
	return &Engine{
		st:        st,
		projects:  projects,
		envs:      envs,
		dataDir:   dataDir,
		wsRoot:    wsRoot,
		depth:     cloneDepth,
		queue:     make(chan string, 64),
		pushQueue: make(chan string, 64),
		locks:     map[string]*sync.Mutex{},
		runCtx:    context.Background(),
	}
}

// Start recovers interrupted jobs and launches the worker plus the retention prune loop.
func (e *Engine) Start(ctx context.Context) error {
	if err := job.RecoverInterrupted(ctx, e.st); err != nil {
		return err
	}
	ids, err := job.QueuedIDs(ctx, e.st)
	if err != nil {
		return err
	}
	runCtx, cancel := context.WithCancel(ctx)
	e.runCtx = runCtx
	e.stop = cancel
	go e.loop(runCtx, e.queue, e.jobLimit)
	go e.loop(runCtx, e.pushQueue, e.jobLimit)
	go e.pruneLoop(runCtx)
	for _, id := range ids {
		e.Enqueue(id)
	}
	return nil
}

// Stop cancels the worker.
func (e *Engine) Stop() {
	if e.stop != nil {
		e.stop()
	}
}

// Enqueue puts a job id on the pack or push queue.
func (e *Engine) Enqueue(id string) {
	dest := e.queue
	if e.st != nil && strings.TrimSpace(id) != "" {
		if j, err := job.JobByID(context.Background(), e.st, id); err == nil {
			dest = e.destQueue(j.JobType)
		}
	}
	select {
	case dest <- id:
	default:
		// 不在 worker 里阻塞入队，避免队列满时与 loop 等 done 互相卡住；
		// 进程退出时丢掉未送出的 id，避免溢出 goroutine 永久阻塞。
		ctx := e.runCtx
		if ctx == nil {
			ctx = context.Background()
		}
		go func() {
			select {
			case dest <- id:
			case <-ctx.Done():
			}
		}()
	}
}

func (e *Engine) destQueue(jobType string) chan string {
	if job.IsPushJob(jobType) && e.pushQueue != nil {
		return e.pushQueue
	}
	return e.queue
}

func (e *Engine) loop(ctx context.Context, q <-chan string, limit func() int) {
	done := make(chan struct{}, settings.MaxJobConcurrency)
	running := 0
	for {
		if running >= limit() {
			select {
			case <-ctx.Done():
				return
			case <-done:
				running--
			}
			continue
		}
		select {
		case <-ctx.Done():
			return
		case <-done:
			running--
		case id := <-q:
			running++
			go func(jobID string) {
				defer func() { done <- struct{}{} }()
				e.runJob(ctx, jobID)
			}(id)
		}
	}
}

func (e *Engine) jobLimit() int {
	if e.settings == nil {
		return settings.DefaultJobConcurrency
	}
	return e.settings.Snapshot(context.Background()).JobConcurrency
}

func (e *Engine) lockKey(j job.Job) string {
	if j.ProjectRecID != "" {
		return "p:" + j.ProjectRecID
	}
	return "w:" + j.WorkspaceRecID
}

func (e *Engine) projectLock(key string) *sync.Mutex {
	e.mu.Lock()
	defer e.mu.Unlock()
	m, ok := e.locks[key]
	if !ok {
		m = &sync.Mutex{}
		e.locks[key] = m
	}
	return m
}

func (e *Engine) runJob(ctx context.Context, id string) {
	j, err := job.JobByID(ctx, e.st, id)
	if err != nil {
		return
	}
	if j.JobStatus == job.StatusCanceled {
		return
	}
	lk := e.projectLock(e.lockKey(j))
	lk.Lock()
	defer lk.Unlock()

	j, err = job.JobByID(ctx, e.st, id)
	if err != nil || j.JobStatus == job.StatusCanceled {
		return
	}
	_ = job.SetJobStatus(ctx, e.st, id, job.StatusRunning, "", true, false)
	logw, closer := e.openLog(id)
	defer closer()

	steps, err := job.StepsByJob(ctx, e.st, id)
	if err != nil {
		_ = job.SetJobStatus(ctx, e.st, id, job.StatusFailed, err.Error(), false, true)
		e.notify(ctx, j, false, err.Error(), logw)
		return
	}
	// 前面步骤失败后仍跑 post，让仅失败 / 始终 的后置能按 when 执行。
	var failErr error
	for _, st := range steps {
		j, _ = job.JobByID(ctx, e.st, id)
		if j.JobStatus == job.StatusCanceled {
			_ = job.SetStepStatus(ctx, e.st, st.RecID, job.StepCanceled, "", false, true)
			return
		}
		if failErr != nil && st.StepKey != "post" {
			_ = job.SetStepStatus(ctx, e.st, st.RecID, job.StepSkipped, "previous step failed", false, true)
			continue
		}
		_ = job.SetStepStatus(ctx, e.st, st.RecID, job.StepRunning, "", true, false)
		logLine(logw, "== %s ==", st.StepName)
		stepStart := time.Now()
		if err := e.execStep(ctx, j, st, logw, failErr == nil); err != nil {
			_ = job.SetStepStatus(ctx, e.st, st.RecID, job.StepFailed, err.Error(), false, true)
			if failErr == nil {
				_ = job.SetJobStatus(ctx, e.st, id, job.StatusFailed, err.Error(), false, true)
				failErr = err
			}
			logLine(logw, "失败 %s: %v（%s）", st.StepName, err, formatElapsed(stepStart))
			continue
		}
		_ = job.SetStepStatus(ctx, e.st, st.RecID, job.StepSucceeded, "", false, true)
		logLine(logw, "%s 耗时 %s", st.StepName, formatElapsed(stepStart))
	}
	if failErr != nil {
		e.notify(ctx, j, false, failErr.Error(), logw)
		return
	}
	_ = job.SetJobStatus(ctx, e.st, id, job.StatusSucceeded, "", false, true)
	e.notify(ctx, j, true, "", logw)
	e.afterSuccess(ctx, j, logw)
}

func (e *Engine) execStep(ctx context.Context, j job.Job, st job.Step, w io.Writer, ok bool) error {
	payload := job.DecodePayload(j.PayloadJSON)
	var prj project.Project
	if j.ProjectRecID != "" && e.projects != nil {
		p, err := e.projects.GetRaw(ctx, j.ProjectRecID)
		if err == nil {
			prj = p
		}
	}
	work := e.workDir(ctx, j, prj, payload)
	switch st.StepKey {
	case "sync":
		return e.stepSync(ctx, j, prj, payload, work, w)
	case "clone":
		if j.JobType == job.TypeBranchClone {
			return e.stepBranchClone(ctx, j, prj, payload, work, w)
		}
		return e.stepSync(ctx, j, prj, payload, work, w)
	case "build":
		return e.stepBuild(ctx, j, prj, payload, work, w)
	case "version_txt":
		return e.stepVersionTxt(work, payload, w)
	case "pom":
		return e.stepPOM(ctx, j, prj, payload, work, w)
	case "tag":
		recipe := project.ParseRecipe(prj.RecipeJSON)
		return git.Tag(ctx, work, payload.Tag, payload.Notes, true, e.gitOptFor(ctx, j.WorkspaceRecID, recipe.GitEnvRecID), w)
	case "ftp":
		return e.stepFTP(ctx, j, prj, payload, work, w)
	case "post":
		return e.stepPost(ctx, j, prj, payload, work, w, ok)
	default:
		return fmt.Errorf("unknown step %s", st.StepKey)
	}
}
