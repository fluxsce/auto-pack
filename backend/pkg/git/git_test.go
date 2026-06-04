package git

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func requireGit(t *testing.T) {
	t.Helper()
	if _, err := Executable(); err != nil {
		t.Skip("git not available:", err)
	}
}

func TestInitStatusCommit(t *testing.T) {
	requireGit(t)
	ctx := context.Background()
	dir := filepath.Join(t.TempDir(), "repo")

	repo, err := Init(ctx, dir, false)
	if err != nil {
		t.Fatal(err)
	}

	st, err := repo.Status(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !st.Clean {
		t.Fatalf("expected clean repo, got %+v", st)
	}

	if err := os.WriteFile(filepath.Join(dir, "hello.txt"), []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}
	st, err = repo.Status(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if st.Clean || len(st.Untracked) != 1 {
		t.Fatalf("status = %+v", st)
	}

	if err := repo.Add(ctx); err != nil {
		t.Fatal(err)
	}
	hash, err := repo.Commit(ctx, "initial", CommitOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if hash == "" {
		t.Fatal("expected commit hash in output")
	}

	head, err := repo.HEAD(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(head) < 40 {
		t.Fatalf("HEAD = %q", head)
	}

	branch, err := repo.CurrentBranch(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if branch == "" {
		t.Fatal("expected default branch name")
	}
}

func TestOpenNotRepository(t *testing.T) {
	requireGit(t)
	dir := t.TempDir()
	_, err := Open(dir)
	if !errors.Is(err, ErrNotRepository) {
		t.Fatalf("got %v, want ErrNotRepository", err)
	}
}

func TestIsRepositoryMissingDir(t *testing.T) {
	requireGit(t)
	dir := filepath.Join(t.TempDir(), "missing", "repo")
	if IsRepository(context.Background(), dir) {
		t.Fatal("expected false for missing directory")
	}
}

func TestRevParseAndShort(t *testing.T) {
	requireGit(t)
	ctx := context.Background()
	dir := filepath.Join(t.TempDir(), "repo")
	repo, err := Init(ctx, dir, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "f.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := repo.Add(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Commit(ctx, "seed", CommitOptions{}); err != nil {
		t.Fatal(err)
	}

	full, err := repo.RevParse(ctx, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	short, err := repo.ShortRev(ctx, "HEAD", 7)
	if err != nil {
		t.Fatal(err)
	}
	if len(short) != 7 || full[:7] != short {
		t.Fatalf("short=%q full=%q", short, full)
	}
}

func TestParsePorcelainStatus(t *testing.T) {
	raw := "## main...origin/main [ahead 2, behind 1]\n M pkg/a.go\n?? tmp.txt\n"
	st := parsePorcelainStatus(raw)
	if st.Branch != "main" || st.Upstream != "origin/main" {
		t.Fatalf("branch=%q upstream=%q", st.Branch, st.Upstream)
	}
	if st.Ahead != 2 || st.Behind != 1 {
		t.Fatalf("ahead=%d behind=%d", st.Ahead, st.Behind)
	}
	if len(st.Modified) != 1 || len(st.Untracked) != 1 {
		t.Fatalf("modified=%v untracked=%v", st.Modified, st.Untracked)
	}
}

func TestRemotesSetURL(t *testing.T) {
	requireGit(t)
	ctx := context.Background()
	dir := filepath.Join(t.TempDir(), "repo")
	repo, err := Init(ctx, dir, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.SetRemoteURL(ctx, "origin", "https://example.com/repo.git"); err != nil {
		t.Fatal(err)
	}
	remotes, err := repo.Remotes(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(remotes) != 1 || remotes[0].URL != "https://example.com/repo.git" {
		t.Fatalf("remotes = %+v", remotes)
	}
	if err := repo.SetRemoteURL(ctx, "origin", "https://example.com/other.git"); err != nil {
		t.Fatal(err)
	}
	remotes, err = repo.Remotes(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if remotes[0].URL != "https://example.com/other.git" {
		t.Fatalf("url = %q", remotes[0].URL)
	}
}
