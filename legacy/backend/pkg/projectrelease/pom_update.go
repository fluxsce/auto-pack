package projectrelease

import (
	"context"
	"fmt"
	"os"
	"strings"

	"auto-pack/backend/internal/repository"
	"auto-pack/backend/pkg/projectcfg"
)

// PomVersionRequest updates pom.xml versions in a project work tree and optionally publishes to Git.
type PomVersionRequest struct {
	WorkDir       string
	Workspace     *repository.PackWorkspace
	DataDir       string
	Branch        string
	Tag           string
	CommitMessage string
	AllowRepeat   bool
}

// PomVersionResult is the outcome for one project pom version update.
type PomVersionResult struct {
	ProjectName string   `json:"project_name"`
	Status      string   `json:"status"` // ok, skipped, failed
	Message     string   `json:"message,omitempty"`
	PomFiles    int      `json:"pom_files,omitempty"`
	Steps       []string `json:"steps,omitempty"`
}

// PomVersionUpdater syncs Git, rewrites pom versions, and commits.
type PomVersionUpdater struct {
	Git *GitCommitter
}

// Run updates pom.xml under workDir and pushes to the project Git branch when Git is configured.
func (u *PomVersionUpdater) Run(ctx context.Context, project *repository.PackProject, req PomVersionRequest) PomVersionResult {
	name := strings.TrimSpace(project.ProjectName)
	res := PomVersionResult{ProjectName: name, Status: "failed"}

	workDir := strings.TrimSpace(req.WorkDir)
	if workDir == "" {
		res.Message = "work directory is empty"
		res.Status = "skipped"
		return res
	}
	tag := strings.TrimSpace(req.Tag)
	if tag == "" {
		res.Message = "version tag is required"
		return res
	}
	if err := ValidateVersionTag(tag); err != nil {
		res.Message = err.Error()
		return res
	}
	if !IsMavenReleaseProject(project.ProjectType) {
		res.Status = "skipped"
		res.Message = "not a Maven project"
		return res
	}

	branch := strings.TrimSpace(req.Branch)
	if branch == "" {
		branch = strings.TrimSpace(project.RepoBranch)
	}
	if branch == "" {
		branch = "main"
	}

	steps := []string{}
	log := newPublishLogger(&steps)
	log.step("开始更新 POM 版本（工程 %s）", name)

	if u.Git != nil && projectcfg.HasRemotePush(project) {
		if req.Workspace != nil && strings.TrimSpace(req.DataDir) != "" {
			if err := u.Git.SyncProjectSourceWorktree(ctx, project, req.Workspace, req.DataDir, branch, log); err != nil {
				res.Message = err.Error()
				res.Steps = steps
				return res
			}
		} else if err := u.Git.SyncProjectBranchWorktree(ctx, project, workDir, branch, log); err != nil {
			res.Message = err.Error()
			res.Steps = steps
			return res
		}
	} else if !projectcfg.HasRemotePush(project) {
		res.Status = "skipped"
		res.Message = "no Git repository URL configured"
		res.Steps = steps
		return res
	}

	pomFiles, pomSteps, err := CollectPomVersionUpdates(workDir, tag)
	if err != nil {
		res.Message = err.Error()
		res.Steps = append(steps, pomSteps...)
		return res
	}
	for _, msg := range pomSteps {
		log.step("%s", msg)
	}
	if len(pomFiles) == 0 {
		res.Status = "skipped"
		res.Message = "no pom.xml changes"
		res.Steps = steps
		return res
	}
	res.PomFiles = len(pomFiles)

	commitMsg := strings.TrimSpace(req.CommitMessage)
	if commitMsg == "" {
		commitMsg = fmt.Sprintf("更新 POM 版本号为 %s（版本 Tag %s）", PomVersionForTag(tag), tag)
	}

	if u.Git != nil {
		if err := u.Git.PublishBranchChanges(ctx, project, workDir, branch, commitMsg, req.AllowRepeat, pomFiles, &steps); err != nil {
			res.Message = err.Error()
			res.Steps = steps
			return res
		}
		log.step("已提交并推送 POM 版本更新")
	} else {
		for path, data := range pomFiles {
			if err := os.WriteFile(path, data, 0o644); err != nil {
				res.Message = fmt.Sprintf("write %s: %v", path, err)
				res.Steps = steps
				return res
			}
		}
		log.step("已更新本地 POM 文件（未配置 Git 发布）")
	}

	res.Status = "ok"
	res.Message = fmt.Sprintf("updated %d pom file(s)", len(pomFiles))
	res.Steps = steps
	return res
}
