package git

import (
	"context"
	"strings"

	"auto-pack/backend/pkg/shell"
)

// FetchOrigin updates refs from remote for the configured repo branch.
// Version tags use a force refspec (+refs/tags/X:refs/tags/X) so the remote
// tag overwrites a stale local one without deleting the tag first.
func FetchOrigin(ctx context.Context, dir string, env []string, remote, repoBranch string) error {
	remote = strings.TrimSpace(remote)
	if remote == "" {
		remote = "origin"
	}
	repoBranch = strings.TrimSpace(repoBranch)
	if tagName := tagNameForFetch(repoBranch); tagName != "" {
		refspec := "+refs/tags/" + tagName + ":refs/tags/" + tagName
		_, err := run(ctx, dir, []string{"fetch", "--progress", remote, refspec}, shell.Options{Env: env})
		return err
	}
	args := []string{"fetch", "--prune", "--progress", remote}
	if repoBranch != "" {
		args = append(args, repoBranch)
	}
	_, err := run(ctx, dir, args, shell.Options{Env: env})
	return err
}

func tagNameForFetch(repoBranch string) string {
	repoBranch = strings.TrimSpace(repoBranch)
	if repoBranch == "" {
		return ""
	}
	if IsTagRef(repoBranch) || IsVersionTagLabel(repoBranch) {
		return NormalizeTagName(repoBranch)
	}
	return ""
}
