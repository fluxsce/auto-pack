package release

import (
	"context"
	"fmt"
	"log"
	"strings"
	stdsync "sync"
	"time"

	"auto-pack/server/internal/batchrun"
	"auto-pack/server/internal/iam"
	"auto-pack/server/internal/project"
	"auto-pack/server/internal/release/order"
	"auto-pack/server/internal/release/record"
	"auto-pack/server/internal/workspace"
)

func (s *Service) runPost(ctx context.Context, actor iam.Actor, ws workspace.Workspace, list []project.Project, body StartRequest, job *record.Job, logPath string, results []Result, released int) {
	opts := job.PostOptions
	if st := s.coord.Status(ws.RecID); st.PostOptions != nil {
		opts = st.PostOptions
	}
	if opts == nil || !opts.AutoBuildAfterRelease || released <= 0 {
		return
	}
	s.coord.SetPhase(job, "post_build")
	s.appendLog(logPath, stamp()+" 发版完成，开始自动构建与推送")
	analysis := order.AnalyzeMavenWorkspace(list, func(prj project.Project) string {
		return s.workDir(ws, prj)
	})
	byName := map[string]project.Project{}
	for _, p := range list {
		byName[p.ProjectName] = p
	}
	var releasedIDs []string
	releasedSet := map[string]struct{}{}
	for _, item := range results {
		if item.Status != record.ResultOK {
			continue
		}
		p, ok := byName[item.ProjectName]
		if !ok {
			continue
		}
		releasedIDs = append(releasedIDs, p.RecID)
		releasedSet[p.RecID] = struct{}{}
	}
	queue := order.Queue(list, analysis, order.QueueOpts{TargetRecIDs: releasedIDs})
	if len(queue) == 0 {
		s.appendLog(logPath, stamp()+" 发版后没有可构建的工程")
		return
	}
	ids := make([]string, 0, len(queue))
	for _, p := range queue {
		ids = append(ids, p.RecID)
	}
	s.appendLog(logPath, stamp()+fmt.Sprintf(" 预同步本次队列 %d 个工程", len(ids)))
	sync, err := s.SyncSourcesLimited(ctx, actor, ws.RecID, ids, func(ev SyncEvent) {
		if line := formatReleaseSyncEvent(ev); line != "" {
			s.appendLog(logPath, line)
		}
	})
	if err != nil && ctx.Err() == nil {
		s.appendLog(logPath, stamp()+" 预同步失败："+err.Error())
	}
	s.appendLog(logPath, stamp()+fmt.Sprintf(" 构建队列 %d 个工程", len(queue)))
	for i, p := range queue {
		s.appendLog(logPath, fmt.Sprintf("  %d. %s", i+1, p.ProjectName))
	}
	ftpOn := opts.AutoFtpPushAfterBuild
	pushSet := map[string]struct{}{}
	for _, id := range opts.FtpPushProjectIDs {
		if id = strings.TrimSpace(id); id != "" {
			pushSet[id] = struct{}{}
		}
	}
	ftpReady := ftpOn && strings.TrimSpace(opts.FtpEnvRecID) != "" && strings.TrimSpace(opts.PushPath) != "" && strings.TrimSpace(opts.Pusher) != "" && strings.TrimSpace(opts.TaskRef) != "" && len(pushSet) > 0
	if ftpOn && !ftpReady {
		s.appendLog(logPath, stamp()+" 已勾选发版后 FTP 推送，但环境/路径/推送人或工程未填完整，已跳过构建后推送")
	}
	s.appendLog(logPath, stamp()+" 按 pom / package.json 依赖分层并行，上游成功后才启动下游")
	var logMu stdsync.Mutex
	writeLog := func(line string) {
		logMu.Lock()
		defer logMu.Unlock()
		s.appendLog(logPath, line)
	}
	writeJobLog := func(chunk string) {
		logMu.Lock()
		defer logMu.Unlock()
		s.appendJobLog(logPath, chunk)
	}
	doneN := 0
	outcomes := batchrun.RunGraph(ctx, queue, analysis, func(runCtx context.Context, prj project.Project) batchrun.Outcome {
		return batchrun.RunBuild(runCtx, s.jobs, actor, ws.RecID, prj.RecID, sync.SyncedReady(prj.ProjectName), func(step string) {
			writeLog(stamp() + fmt.Sprintf(" %s · %s", prj.ProjectName, step))
		}, writeJobLog)
	}, batchrun.GraphHooks{
		OnParallel: func(started []project.Project) {
			names := make([]string, 0, len(started))
			for _, p := range started {
				names = append(names, p.ProjectName)
			}
			writeLog(stamp() + " 并行启动：" + strings.Join(names, "、"))
		},
		OnStart: func(prj project.Project) {
			s.coord.SetProgress(job, prj.ProjectName, doneN)
			writeLog(stamp() + fmt.Sprintf(" 正在构建：%s", prj.ProjectName))
		},
		OnSkip: func(prj project.Project, reason string) {
			doneN++
			writeLog(stamp() + fmt.Sprintf(" 跳过构建：%s — %s", prj.ProjectName, reason))
		},
		OnFinish: func(prj project.Project, out batchrun.Outcome, took time.Duration) {
			doneN++
			if out.OK {
				writeLog(stamp() + fmt.Sprintf(" 构建成功：%s（%s）", prj.ProjectName, formatElapsedDur(took)))
				return
			}
			writeLog(stamp() + fmt.Sprintf(" 构建失败：%s — %s（%s）", prj.ProjectName, out.Message, formatElapsedDur(took)))
		},
		OnCancel: func(prj project.Project, err error) {
			doneN++
			writeLog(stamp() + " 发版后流程已取消：" + prj.ProjectName)
		},
	})
	if ftpReady {
		s.coord.SetPhase(job, "post_push")
		for _, prj := range queue {
			if outcomes[prj.RecID] != order.OutcomeOK {
				continue
			}
			if _, ok := releasedSet[prj.RecID]; !ok {
				continue
			}
			if _, ok := pushSet[prj.RecID]; !ok {
				s.appendLog(logPath, stamp()+" 跳过推送："+prj.ProjectName+"（未选入推送清单）")
				continue
			}
			s.appendLog(logPath, stamp()+" 正在推送："+prj.ProjectName)
			push := batchrun.RunPush(ctx, s.jobs, actor, ws.RecID, prj.RecID, batchrun.PushSpec{
				EnvRecID:   opts.FtpEnvRecID,
				PushPath:   opts.PushPath,
				BackupPath: opts.BackupPath,
				Pusher:     opts.Pusher,
				TaskRef:    opts.TaskRef,
				Detail:     opts.Detail,
				Notes:      strings.TrimSpace(strings.TrimSpace(opts.TaskRef) + " | " + strings.TrimSpace(opts.Detail)),
			}, nil, func(chunk string) {
				s.appendJobLog(logPath, chunk)
			})
			if push.OK {
				s.appendLog(logPath, stamp()+" 推送成功："+prj.ProjectName)
			} else {
				s.appendLog(logPath, stamp()+" 推送失败："+prj.ProjectName+" — "+push.Message)
			}
		}
	}
	s.appendLog(logPath, stamp()+" 自动构建与推送流程结束")
}

func (s *Service) appendLog(path, line string) {
	if err := record.AppendLog(path, line); err != nil {
		log.Printf("release log %s: %v", path, err)
	}
}

func (s *Service) appendJobLog(path, chunk string) {
	chunk = strings.TrimRight(chunk, "\r\n")
	if chunk == "" {
		return
	}
	for _, line := range strings.Split(chunk, "\n") {
		line = strings.TrimRight(line, "\r")
		if strings.TrimSpace(line) == "" {
			continue
		}
		s.appendLog(path, "  "+line)
	}
}

func stamp() string {
	return "[" + time.Now().Format("2006-01-02 15:04:05") + "]"
}

func formatElapsedDur(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	if d >= time.Second {
		return d.Truncate(time.Second).String()
	}
	return d.Truncate(time.Millisecond).String()
}

func formatReleaseSyncEvent(ev SyncEvent) string {
	switch ev.Event {
	case "start":
		return stamp() + fmt.Sprintf(" 预同步开始，共 %d 个工程", ev.Total)
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

