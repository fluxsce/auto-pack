package git

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestTagNameForFetch(t *testing.T) {
	tests := []struct {
		branch string
		want   string
	}{
		{"refs/tags/V9.0.0-P01-01", "V9.0.0-P01-01"},
		{"V9.0.0-P01-01", "V9.0.0-P01-01"},
		{"release/V9.0.0-P01", ""},
		{"main", ""},
		{"", ""},
	}
	for _, tc := range tests {
		got := tagNameForFetch(tc.branch)
		if got != tc.want {
			t.Fatalf("tagNameForFetch(%q)=%q want %q", tc.branch, got, tc.want)
		}
	}
}

func TestFetchOrigin_tagForceOverwrite(t *testing.T) {
	requireGit(t)
	ctx := context.Background()
	root := t.TempDir()
	bare := filepath.Join(root, "remote.git")
	cloneDir := filepath.Join(root, "clone")

	runGit := func(dir string, args ...string) {
		t.Helper()
		cmd := append([]string{"-C", dir}, args...)
		if out, err := exec.CommandContext(ctx, "git", cmd...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
	}

	runGit("", "init", "--bare", bare)
	runGit("", "clone", bare, cloneDir)
	if err := os.WriteFile(filepath.Join(cloneDir, "f.txt"), []byte("v1"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(cloneDir, "add", "f.txt")
	runGit(cloneDir, "commit", "-m", "v1")
	runGit(cloneDir, "tag", "V10.0.0-P01")
	runGit(cloneDir, "push", "origin", "V10.0.0-P01")

	workDir := filepath.Join(root, "work")
	runGit("", "clone", bare, workDir)
	oldTagCommit := gitRevParse(t, workDir, "V10.0.0-P01")

	if err := os.WriteFile(filepath.Join(cloneDir, "f.txt"), []byte("v2"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(cloneDir, "commit", "-am", "v2")
	newTagCommit := gitRevParse(t, cloneDir, "HEAD")
	runGit(cloneDir, "tag", "-f", "V10.0.0-P01")
	runGit(cloneDir, "push", "--force", "origin", "refs/tags/V10.0.0-P01")

	if oldTagCommit == newTagCommit {
		t.Fatal("test setup: tag commits should differ")
	}
	if got := gitRevParse(t, workDir, "V10.0.0-P01"); got != oldTagCommit {
		t.Fatalf("worktree should keep stale tag before fetch")
	}

	if err := FetchOrigin(ctx, workDir, nil, "origin", "refs/tags/V10.0.0-P01"); err != nil {
		t.Fatal(err)
	}
	if got := gitRevParse(t, workDir, "V10.0.0-P01"); got != newTagCommit {
		t.Fatalf("tag not force-updated: got %s want %s", got, newTagCommit)
	}
}

func gitRevParse(t *testing.T, dir, ref string) string {
	t.Helper()
	out, err := exec.Command("git", "-C", dir, "rev-parse", ref).CombinedOutput()
	if err != nil {
		t.Fatalf("rev-parse %s: %v %s", ref, err, out)
	}
	return strings.TrimSpace(string(out))
}
