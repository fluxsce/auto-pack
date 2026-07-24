package git

import (
	"context"
	"strings"

	"auto-pack/backend/pkg/shell"
)

// RemoteTrackingRef returns refs/remotes/{remote}/{branch}.
func RemoteTrackingRef(remote, branch string) string {
	remote = strings.TrimSpace(remote)
	if remote == "" {
		remote = "origin"
	}
	branch = strings.TrimSpace(branch)
	if branch == "" {
		return ""
	}
	return "refs/remotes/" + remote + "/" + branch
}

// OriginBranchRef returns origin/BRANCH.
func OriginBranchRef(branch string) string {
	branch = strings.TrimSpace(branch)
	if branch == "" {
		return ""
	}
	return "origin/" + branch
}

// FetchOriginArgs builds git fetch arguments for the configured repo branch.
// Branch tip sync respects GlobalCloneDepth (app.yaml git.clone_depth).
// Tag refspecs intentionally omit --depth so release checkouts can still reach
// version tags that sit outside the shallow window of the branch tip.
func FetchOriginArgs(remote, repoBranch string) []string {
	remote = strings.TrimSpace(remote)
	if remote == "" {
		remote = "origin"
	}
	repoBranch = strings.TrimSpace(repoBranch)
	if tagName := tagNameForFetch(repoBranch); tagName != "" {
		refspec := "+refs/tags/" + tagName + ":refs/tags/" + tagName
		// No AppendFetchDepthArgs: old release tags must remain fetchable.
		return []string{"fetch", "--progress", remote, refspec}
	}
	args := []string{"fetch", "--prune", "--progress", remote}
	args = AppendFetchDepthArgs(args)
	if repoBranch != "" && !IsTagRef(repoBranch) {
		args = append(args, "refs/heads/"+repoBranch+":"+RemoteTrackingRef(remote, repoBranch))
	}
	return args
}

// FetchOrigin updates refs from remote for the configured repo branch.
// Branches use an explicit refspec so origin/BRANCH exists after fetch (Git 1.8+).
// Version tags use a force refspec (+refs/tags/X:refs/tags/X) so the remote
// tag overwrites a stale local one without deleting the tag first.
func FetchOrigin(ctx context.Context, dir string, env []string, remote, repoBranch string) error {
	remote = strings.TrimSpace(remote)
	if remote == "" {
		remote = "origin"
	}
	repoBranch = strings.TrimSpace(repoBranch)
	args := FetchOriginArgs(remote, repoBranch)
	_, err := run(ctx, dir, args, shell.Options{Env: env})
	if err == nil || repoBranch == "" || tagNameForFetch(repoBranch) != "" {
		return err
	}
	fallback := AppendFetchDepthArgs([]string{"fetch", "--prune", "--progress", remote, repoBranch})
	_, err2 := run(ctx, dir, fallback, shell.Options{Env: env})
	return err2
}

func tagNameForFetch(repoBranch string) string {
	if !IsTagRef(repoBranch) {
		return ""
	}
	return NormalizeTagName(repoBranch)
}
