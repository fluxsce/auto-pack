package projectrelease

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"auto-pack/backend/internal/repository"
	"auto-pack/backend/pkg/redact"
)

// ReleaseRequest is input for one project release.
type ReleaseRequest struct {
	WorkDir            string
	ReleaseBranch      string
	Tag                string
	ReleaseNote        string
	ReleaseDate        string // YYYY-MM-DD, empty => today
	AllowRepeat        bool
	UpdatePomVersion   bool
	CreateTargetBranch bool
	TargetBranch       string // e.g. release/V9.0.0-P08
}

// ReleaseResult describes the outcome for one project.
type ReleaseResult struct {
	ProjectName string   `json:"project_name"`
	Status      string   `json:"status"` // ok, skipped, failed
	Message     string   `json:"message,omitempty"`
	Steps       []string `json:"steps"`
}

// Releaser runs version sync and optional git publish per project.
type Releaser struct {
	Git *GitCommitter
}

// RunRelease updates Version.txt from UpdateNotes and pushes to the release branch.
func (r *Releaser) RunRelease(ctx context.Context, project *repository.PackProject, req ReleaseRequest) ReleaseResult {
	name := strings.TrimSpace(project.ProjectName)
	res := ReleaseResult{ProjectName: name, Status: "failed"}

	workDir := strings.TrimSpace(req.WorkDir)
	if workDir == "" {
		res.Message = "work directory is empty"
		return res
	}

	tag := strings.TrimSpace(req.Tag)
	if tag == "" {
		res.Message = "tag branch is required"
		return res
	}
	releaseDate := strings.TrimSpace(req.ReleaseDate)
	if releaseDate == "" {
		releaseDate = time.Now().Format("2006-01-02")
	}
	branch := strings.TrimSpace(req.ReleaseBranch)
	if branch == "" {
		res.Message = "release branch is required"
		return res
	}
	commitMsg := ResolveCommitMessage(req.ReleaseNote, tag)

	steps := []string{}
	log := newPublishLogger(&steps)
	log.step("开始发版（工程 %s）", name)

	// Sync from origin before reading or creating resource files — avoid polluted local branches.
	if r.Git != nil {
		if err := r.Git.SyncReleaseWorktree(ctx, project, workDir, branch, tag, log); err != nil {
			res.Message = redact.Redact(err.Error())
			res.Steps = steps
			return res
		}
	}

	versionPath, updateNotesPath, ok := ResolveResourcePaths(workDir)
	if !ok {
		res.Message = "invalid project work directory"
		res.Steps = steps
		return res
	}
	if _, err := EnsureResourceFiles(versionPath, updateNotesPath, log); err != nil {
		res.Message = redact.Redact(err.Error())
		res.Steps = steps
		return res
	}

	if baseVer := topVersionFromFile(versionPath); baseVer != "" {
		log.step("Version.txt 以发版分支内容为准（当前首行版本 %s）", baseVer)
	}

	content, err := BuildVersionContent(versionPath, updateNotesPath, SyncOptions{
		Tag:         tag,
		ReleaseDate: releaseDate,
		AllowRepeat: req.AllowRepeat,
	})
	if err != nil {
		res.Message = redact.Redact(err.Error())
		res.Steps = steps
		return res
	}
	log.step("已生成 Version.txt（版本 %s，发布日期 %s）", tag, releaseDate)

	files := map[string][]byte{versionPath: []byte(content)}
	if req.UpdatePomVersion {
		pomFiles, pomSteps, err := CollectPomVersionUpdates(workDir, tag)
		if err != nil {
			res.Message = redact.Redact(err.Error())
			res.Steps = steps
			return res
		}
		for path, data := range pomFiles {
			files[path] = data
		}
		for _, msg := range pomSteps {
			log.step("%s", msg)
		}
	}

	if r.Git == nil {
		for path, data := range files {
			if err := os.WriteFile(path, data, 0o644); err != nil {
				res.Message = fmt.Sprintf("write %s: %v", path, err)
				res.Steps = steps
				return res
			}
		}
		res.Status = "ok"
		res.Message = "files updated (no git publish)"
		res.Steps = steps
		return res
	}

	targetBranch := strings.TrimSpace(req.TargetBranch)
	if req.CreateTargetBranch && targetBranch == "" {
		res.Message = "target branch is required when creating target release branch"
		res.Steps = steps
		return res
	}
	if err := r.Git.PublishChanges(
		ctx, project, workDir, branch, tag, targetBranch, req.CreateTargetBranch, commitMsg, req.AllowRepeat, files, &steps,
	); err != nil {
		res.Message = redact.Redact(err.Error())
		res.Steps = steps
		return res
	}
	res.Status = "ok"
	res.Message = formatReleaseMessage(branch, tag, req.CreateTargetBranch, targetBranch)
	res.Steps = steps
	return res
}

func topVersionFromFile(versionPath string) string {
	data, err := os.ReadFile(versionPath)
	if err != nil {
		return ""
	}
	blocks := ParseVersionFile(string(data))
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
