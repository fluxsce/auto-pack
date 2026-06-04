package git

import (
	"context"
	"fmt"
	"strings"

	"auto-pack/backend/pkg/shell"
)

// Remote describes a configured remote.
type Remote struct {
	Name string
	URL  string
}

// CurrentBranch returns the active branch name, or empty when detached.
func (r *Repo) CurrentBranch(ctx context.Context) (string, error) {
	out, err := run(ctx, r.Dir, []string{"rev-parse", "--abbrev-ref", "HEAD"}, shell.Options{})
	if err != nil {
		return "", err
	}
	branch := trimLine(out.Stdout)
	if branch == "HEAD" {
		return "", nil
	}
	return branch, nil
}

// HEAD returns the full commit hash at HEAD.
func (r *Repo) HEAD(ctx context.Context) (string, error) {
	return r.RevParse(ctx, "HEAD")
}

// RevParse resolves ref to a commit hash (or other object name git resolves).
func (r *Repo) RevParse(ctx context.Context, ref string) (string, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return "", fmt.Errorf("git: empty ref")
	}
	out, err := run(ctx, r.Dir, []string{"rev-parse", ref}, shell.Options{})
	if err != nil {
		return "", err
	}
	return trimLine(out.Stdout), nil
}

// ShortRev returns a short hash for ref (default 7 characters).
func (r *Repo) ShortRev(ctx context.Context, ref string, length int) (string, error) {
	if length <= 0 {
		length = 7
	}
	ref = strings.TrimSpace(ref)
	if ref == "" {
		ref = "HEAD"
	}
	out, err := run(ctx, r.Dir, []string{"rev-parse", fmt.Sprintf("--short=%d", length), ref}, shell.Options{})
	if err != nil {
		return "", err
	}
	return trimLine(out.Stdout), nil
}

// ListBranches returns local or remote branch names.
func (r *Repo) ListBranches(ctx context.Context, remote bool) ([]string, error) {
	args := []string{"branch", "--format=%(refname:short)"}
	if remote {
		args = []string{"branch", "-r", "--format=%(refname:short)"}
	}
	out, err := run(ctx, r.Dir, args, shell.Options{})
	if err != nil {
		return nil, err
	}
	var names []string
	for _, line := range lines(out.Stdout) {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasSuffix(line, "/HEAD") {
			continue
		}
		if remote {
			line = strings.TrimPrefix(line, "origin/")
		}
		names = append(names, line)
	}
	return names, nil
}

// Remotes lists configured remotes and their fetch URLs.
func (r *Repo) Remotes(ctx context.Context) ([]Remote, error) {
	out, err := run(ctx, r.Dir, []string{"remote", "-v"}, shell.Options{})
	if err != nil {
		return nil, err
	}
	seen := make(map[string]Remote)
	for _, line := range lines(out.Stdout) {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		if len(fields) >= 3 && fields[2] != "(fetch)" {
			continue
		}
		name, url := fields[0], fields[1]
		if _, ok := seen[name]; !ok {
			seen[name] = Remote{Name: name, URL: url}
		}
	}
	outList := make([]Remote, 0, len(seen))
	for _, rm := range seen {
		outList = append(outList, rm)
	}
	return outList, nil
}

// SetRemoteURL sets the URL for a remote (creates the remote if missing).
func (r *Repo) SetRemoteURL(ctx context.Context, name, url string) error {
	name = strings.TrimSpace(name)
	url = strings.TrimSpace(url)
	if name == "" || url == "" {
		return fmt.Errorf("git: remote name and url required")
	}
	remotes, err := r.Remotes(ctx)
	if err != nil {
		return err
	}
	found := false
	for _, rm := range remotes {
		if rm.Name == name {
			found = true
			break
		}
	}
	if found {
		_, err = run(ctx, r.Dir, []string{"remote", "set-url", name, url}, shell.Options{})
	} else {
		_, err = run(ctx, r.Dir, []string{"remote", "add", name, url}, shell.Options{})
	}
	return err
}
