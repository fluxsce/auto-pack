package git

import (
	"context"
	"fmt"
	"strings"

	"auto-pack/backend/pkg/shell"
)

// ParseLsRemoteTags extracts tag names from `git ls-remote --tags` output.
func ParseLsRemoteTags(stdout string) []string {
	seen := make(map[string]struct{})
	var tags []string
	for _, line := range strings.Split(stdout, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		ref := fields[1]
		if !strings.HasPrefix(ref, "refs/tags/") {
			continue
		}
		name := strings.TrimPrefix(ref, "refs/tags/")
		if strings.HasSuffix(name, "^{}") {
			continue
		}
		name = NormalizeTagName(name)
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		tags = append(tags, name)
	}
	return tags
}

// ListRemoteTagsOnOrigin lists tags on origin via ls-remote (no local fetch required).
func ListRemoteTagsOnOrigin(ctx context.Context, dir string, env []string) ([]string, error) {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return nil, fmt.Errorf("git: work directory is empty")
	}
	if !IsRepository(ctx, dir) {
		return nil, ErrNotRepository
	}
	exe, err := Executable()
	if err != nil {
		return nil, err
	}
	res, err := shell.RunArgs(ctx, exe, []string{"ls-remote", "--tags", "origin"}, shell.Options{
		Dir:              dir,
		Env:              env,
		AllowNonZeroExit: true,
	})
	if err != nil {
		return nil, err
	}
	if res.ExitCode != 0 {
		msg := strings.TrimSpace(res.Stderr)
		if msg == "" {
			msg = "git ls-remote --tags failed"
		}
		return nil, fmt.Errorf("%s", msg)
	}
	return ParseLsRemoteTags(res.Stdout), nil
}

// ListRemoteTagsOnURL lists tags on a remote URL (for repos without a local clone).
func ListRemoteTagsOnURL(ctx context.Context, repoURL string, env []string) ([]string, error) {
	repoURL = strings.TrimSpace(repoURL)
	if repoURL == "" {
		return nil, fmt.Errorf("git: repository URL is empty")
	}
	exe, err := Executable()
	if err != nil {
		return nil, err
	}
	res, err := shell.RunArgs(ctx, exe, []string{"ls-remote", "--tags", repoURL}, shell.Options{
		Env:              env,
		AllowNonZeroExit: true,
	})
	if err != nil {
		return nil, err
	}
	if res.ExitCode != 0 {
		msg := strings.TrimSpace(res.Stderr)
		if msg == "" {
			msg = "git ls-remote --tags failed"
		}
		return nil, fmt.Errorf("%s", msg)
	}
	return ParseLsRemoteTags(res.Stdout), nil
}
