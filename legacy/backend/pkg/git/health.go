package git

import (
	"context"
	"strings"

	"auto-pack/backend/pkg/shell"
)

// RefExists reports whether ref resolves in the repository at dir.
func RefExists(ctx context.Context, dir string, env []string, ref string) bool {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return false
	}
	exe, err := Executable()
	if err != nil {
		return false
	}
	res, err := shell.RunArgs(ctx, exe, []string{"rev-parse", "--verify", ref}, shell.Options{
		Dir:              dir,
		Env:              env,
		AllowNonZeroExit: true,
	})
	return err == nil && res.ExitCode == 0
}

// HasResolvableHEAD reports whether HEAD points to a valid object (complete checkout).
func HasResolvableHEAD(ctx context.Context, dir string, env []string) bool {
	return RefExists(ctx, dir, env, "HEAD")
}

// IsCheckoutPathspecError reports git checkout failures from missing refs.
func IsCheckoutPathspecError(msg string) bool {
	lower := strings.ToLower(msg)
	return strings.Contains(lower, "pathspec") && strings.Contains(lower, "did not match")
}

// IsRecoverableSyncError is true when removing the local tree and re-cloning may help
// (e.g. interrupted clone left a broken worktree).
func IsRecoverableSyncError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	if IsCheckoutPathspecError(msg) {
		return true
	}
	return strings.Contains(msg, "not a git repository") ||
		strings.Contains(msg, "bad object") ||
		strings.Contains(msg, "does not have any commits") ||
		strings.Contains(msg, "invalid reference")
}
