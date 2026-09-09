package release

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"auto-pack/server/internal/project"
	"auto-pack/server/internal/release/notes"
	"auto-pack/server/internal/release/pom"
	"auto-pack/server/internal/release/publish"
	"auto-pack/server/internal/release/record"
)

// Releaser runs version sync and optional git publish per project.
type Releaser struct {
	Git *publish.GitCommitter
}

// Run updates Version.txt from UpdateNotes and publishes to the release branch.
func (r *Releaser) Run(ctx context.Context, prj project.Project, req Request) Result {
	name := strings.TrimSpace(prj.ProjectName)
	res := Result{ProjectName: name, Status: record.ResultFailed}

	workDir := strings.TrimSpace(req.WorkDir)
	if workDir == "" {
		return failResult(res, nil, "work directory is empty")
	}
	tag := strings.TrimSpace(req.Tag)
	if tag == "" {
		return failResult(res, nil, "tag branch is required")
	}
	releaseDate := strings.TrimSpace(req.ReleaseDate)
	if releaseDate == "" {
		releaseDate = time.Now().Format("2006-01-02")
	}
	branch := strings.TrimSpace(req.ReleaseBranch)
	if branch == "" {
		return failResult(res, nil, "release branch is required")
	}
	commitMsg := notes.ResolveCommitMessage(req.ReleaseNote, tag)

	steps := []string{}
	log := publish.NewLogger(&steps)
	log.Step("开始发版（工程 %s）", name)

	projectType := strings.TrimSpace(prj.ProjectType)
	if !notes.SupportsAutoReleaseVersionSync(projectType) {
		res.Status = record.ResultSkipped
		res.Message = fmt.Sprintf("工程类型 %s 不支持自动发版版本文件同步", projectType)
		res.Steps = steps
		return res
	}
	if notes.IsNodeReleaseProject(projectType) {
		log.Step("Node/Vue 工程：使用 src/resources/updateNotes/updateNotes.txt → Version.txt")
	} else {
		log.Step("Maven 工程：使用 src/main/resources/updatenotes/UpdateNotes.txt → Version.txt")
	}

	if r.Git != nil {
		if err := r.Git.SyncReleaseWorktree(ctx, prj, workDir, branch, tag, log); err != nil {
			return failResult(res, steps, err.Error())
		}
	}

	versionPath, updateNotesPath, ok := notes.ResolveResourcePaths(workDir, projectType)
	if !ok {
		return failResult(res, steps, "invalid project work directory")
	}
	created, err := notes.EnsureResourceFiles(versionPath, updateNotesPath)
	if err != nil {
		return failResult(res, steps, err.Error())
	}
	if len(created) > 0 {
		log.Step("已自动创建 Version.txt/UpdateNotes.txt")
	}

	if baseVer := topVersionFromFile(versionPath); baseVer != "" {
		log.Step("Version.txt 以发版分支内容为准（当前首行版本 %s）", baseVer)
	}

	content, err := notes.BuildVersionContent(versionPath, updateNotesPath, notes.SyncOptions{
		Tag:         tag,
		ReleaseDate: releaseDate,
		AllowRepeat: req.AllowRepeat,
	})
	if err != nil {
		return failResult(res, steps, err.Error())
	}
	log.Step("已生成 Version.txt（版本 %s，发布日期 %s）", tag, releaseDate)

	files := map[string][]byte{versionPath: []byte(content)}
	targetBranch := strings.TrimSpace(req.TargetBranch)
	pomOnTarget := publish.DeferPomToTargetBranch(req.CreateTargetBranch, branch, targetBranch)
	if req.UpdatePomVersion && notes.IsMavenReleaseProject(projectType) {
		pomFiles, pomSteps, err := pom.CollectPomVersionUpdates(workDir, tag)
		if err != nil {
			return failResult(res, steps, err.Error())
		}
		for path, data := range pomFiles {
			files[path] = data
		}
		for _, msg := range pomSteps {
			log.Step("%s", msg)
		}
		if len(pomFiles) > 0 && pomOnTarget {
			log.Step("POM 版本将仅提交到目标分支 %s（发版分支 %s 不更新 pom.xml）", targetBranch, branch)
		}
	} else if req.UpdatePomVersion {
		log.Step("跳过 pom.xml 版本更新（非 Maven 工程）")
	}

	if r.Git == nil {
		for path, data := range files {
			if err := os.WriteFile(path, data, 0o644); err != nil {
				return failResult(res, steps, fmt.Sprintf("write %s: %v", path, err))
			}
		}
		res.Status = record.ResultOK
		res.Message = "files updated (no git publish)"
		res.Steps = steps
		return res
	}

	if req.CreateTargetBranch && targetBranch == "" {
		return failResult(res, steps, "target branch is required when creating target release branch")
	}
	if err := r.Git.PublishChanges(ctx, prj, workDir, branch, tag, targetBranch, req.CreateTargetBranch, commitMsg, req.AllowRepeat, files, &steps); err != nil {
		return failResult(res, steps, err.Error())
	}
	res.Status = record.ResultOK
	res.Message = formatReleaseMessage(branch, tag, req.CreateTargetBranch, targetBranch)
	res.Steps = steps
	return res
}

func topVersionFromFile(versionPath string) string {
	data, err := os.ReadFile(versionPath)
	if err != nil {
		return ""
	}
	blocks := notes.ParseVersionFile(string(data))
	if len(blocks) == 0 {
		return ""
	}
	return strings.TrimSpace(blocks[0].Version)
}

func formatReleaseMessage(releaseBranch, versionTag string, createTarget bool, targetBranch string) string {
	releaseBranch = strings.TrimSpace(releaseBranch)
	versionTag = strings.TrimSpace(versionTag)
	targetBranch = strings.TrimSpace(targetBranch)
	var parts []string
	parts = append(parts, "released to "+releaseBranch)
	if createTarget && targetBranch != "" && !strings.EqualFold(targetBranch, releaseBranch) {
		parts = append(parts, "target branch "+targetBranch)
	}
	if versionTag != "" {
		parts = append(parts, "git tag "+versionTag)
	}
	return strings.Join(parts, ", ")
}

func failResult(res Result, steps []string, message string) Result {
	res.Status = record.ResultFailed
	res.Message = message
	res.Steps = steps
	return res
}
