package git

import (
	"context"
	"fmt"
	"os"
	"strings"

	"auto-pack/backend/pkg/envtypes"
	"auto-pack/backend/pkg/probe"
	"auto-pack/backend/pkg/shell"
)

// VerifyEnv checks git binary availability and optional user.name/email.
func VerifyEnv(ctx context.Context, cfg envtypes.GitConfig) probe.Result {
	details := map[string]string{}
	exe := strings.TrimSpace(cfg.GitPath)
	if exe == "" {
		var err error
		exe, err = Executable()
		if err != nil {
			return probe.Fail(err.Error(), details)
		}
	}
	details["git_path"] = exe

	res, err := shell.RunArgs(ctx, exe, []string{"--version"}, shell.Options{})
	if err != nil {
		return probe.Fail(fmt.Sprintf("git --version failed: %v", err), details)
	}
	details["version"] = strings.TrimSpace(res.Stdout + res.Stderr)

	if u := strings.TrimSpace(cfg.Username); u != "" {
		details["user.name"] = u
	}
	if e := strings.TrimSpace(cfg.Email); e != "" {
		details["user.email"] = e
	}
	opts := shell.Options{}
	if cfg.Username != "" || cfg.Email != "" {
		opts.Env = gitEnvOverrides(cfg)
	}
	if len(opts.Env) > 0 {
		if res, err := shell.RunArgs(ctx, exe, []string{"config", "user.name"}, opts); err == nil {
			details["configured_user.name"] = strings.TrimSpace(res.Stdout)
		}
	}
	auth := strings.ToLower(strings.TrimSpace(cfg.AuthType))
	if auth == "" {
		auth = envtypes.GitAuthNone
	}
	details["auth_type"] = auth
	switch auth {
	case envtypes.GitAuthHTTPS:
		if strings.TrimSpace(cfg.AuthToken) != "" {
			details["auth"] = "https (token)"
		} else if strings.TrimSpace(cfg.AuthUsername) != "" {
			details["auth"] = "https (username/password)"
		}
	case envtypes.GitAuthSSH:
		if p := strings.TrimSpace(cfg.PrivateKeyPath); p != "" {
			details["auth"] = "ssh (" + p + ")"
		}
	}
	return probe.OK("Git available", details)
}

func gitEnvOverrides(cfg envtypes.GitConfig) []string {
	var env []string
	if u := strings.TrimSpace(cfg.Username); u != "" {
		env = append(env, "GIT_AUTHOR_NAME="+u, "GIT_COMMITTER_NAME="+u)
	}
	if e := strings.TrimSpace(cfg.Email); e != "" {
		env = append(env, "GIT_AUTHOR_EMAIL="+e, "GIT_COMMITTER_EMAIL="+e)
	}
	return append(os.Environ(), env...)
}
