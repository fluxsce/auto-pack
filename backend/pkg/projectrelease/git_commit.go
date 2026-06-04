package projectrelease

import (
	"context"
	"fmt"
	"strings"

	"auto-pack/backend/pkg/envtypes"
	"auto-pack/backend/pkg/shell"
)

func applyGitIdentity(env []string, cfg envtypes.GitConfig) []string {
	if u := strings.TrimSpace(cfg.Username); u != "" {
		env = append(env, "GIT_AUTHOR_NAME="+u, "GIT_COMMITTER_NAME="+u)
	}
	if e := strings.TrimSpace(cfg.Email); e != "" {
		env = append(env, "GIT_AUTHOR_EMAIL="+e, "GIT_COMMITTER_EMAIL="+e)
	}
	return env
}

func ensureGitIdentity(env []string) []string {
	hasName := false
	hasEmail := false
	for _, e := range env {
		if strings.HasPrefix(e, "GIT_AUTHOR_NAME=") || strings.HasPrefix(e, "GIT_COMMITTER_NAME=") {
			hasName = true
		}
		if strings.HasPrefix(e, "GIT_AUTHOR_EMAIL=") || strings.HasPrefix(e, "GIT_COMMITTER_EMAIL=") {
			hasEmail = true
		}
	}
	if !hasName {
		env = append(env, "GIT_AUTHOR_NAME=auto-pack", "GIT_COMMITTER_NAME=auto-pack")
	}
	if !hasEmail {
		env = append(env, "GIT_AUTHOR_EMAIL=auto-pack@local", "GIT_COMMITTER_EMAIL=auto-pack@local")
	}
	return env
}

// hasStagedChanges reports whether the index has staged diffs (git diff --cached --quiet exits 1).
func hasStagedChanges(ctx context.Context, dir string, env []string) bool {
	res, err := runGitAllow(ctx, dir, env, "diff", "--cached", "--quiet")
	return err == nil && res.ExitCode == 1
}

func gitCommitRelease(ctx context.Context, dir string, env []string, commitMsg string, log *publishLogger) error {
	commitMsg = strings.TrimSpace(commitMsg)
	if commitMsg == "" {
		commitMsg = "auto-pack release"
	}
	if hasStagedChanges(ctx, dir, env) {
		log.step("git commit（含发版文件变更）")
		res, err := runGitCommit(ctx, dir, env, false, commitMsg)
		if err == nil && res.ExitCode == 0 {
			return nil
		}
		log.step("常规提交未成功（exit %d），改用 git commit --allow-empty", res.ExitCode)
	} else {
		log.step("发版文件与当前分支一致，git commit --allow-empty")
	}
	res, err := runGitCommit(ctx, dir, env, true, commitMsg)
	if err != nil {
		return commitResultErr("commit --allow-empty", res, err)
	}
	if res.ExitCode != 0 {
		return commitResultErr("commit --allow-empty", res, nil)
	}
	return nil
}

func runGitCommit(ctx context.Context, dir string, env []string, allowEmpty bool, message string) (shell.Result, error) {
	args := []string{"-c", "commit.gpgsign=false", "commit"}
	if allowEmpty {
		args = append(args, "--allow-empty")
	}
	args = append(args, "-m", message)
	return runGitAllow(ctx, dir, env, args...)
}

func commitResultErr(subcmd string, res shell.Result, err error) error {
	detail := strings.TrimSpace(res.Stderr)
	if detail == "" {
		detail = strings.TrimSpace(res.Stdout)
	}
	if detail == "" && err != nil {
		return fmt.Errorf("git %s failed (exit %d): %w", subcmd, res.ExitCode, err)
	}
	if detail == "" {
		return fmt.Errorf("git %s failed (exit %d)", subcmd, res.ExitCode)
	}
	return fmt.Errorf("git %s: %s", subcmd, detail)
}
