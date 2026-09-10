package git

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestFetchPullSwitchesSingleBranchClone(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not in PATH")
	}
	root := t.TempDir()
	src := filepath.Join(root, "src")
	origin := filepath.Join(root, "origin.git")
	work := filepath.Join(root, "work")
	mustGit(t, "", "init", "-b", "main", src)
	mustWrite(t, filepath.Join(src, "README.md"), "main\n")
	mustGit(t, src, "add", "README.md")
	mustGit(t, src, "commit", "-m", "init")
	mustGit(t, src, "branch", "master")
	mustGit(t, src, "tag", "-a", "v9.9.9", "-m", "pollute")
	mustGit(t, "", "clone", "--bare", src, origin)
	mustGit(t, "", "clone", "--depth", "1", "--branch", "main", "--no-tags", origin, work)

	if err := runGit(t, work, "checkout", "master"); err == nil {
		t.Fatal("expected pathspec failure on single-branch clone")
	}

	var buf bytes.Buffer
	if err := FetchPull(context.Background(), work, "master", Options{Depth: 1}, &buf); err != nil {
		t.Fatalf("FetchPull: %v\n%s", err, buf.String())
	}
	head := strings.TrimSpace(gitOut(t, work, "rev-parse", "--abbrev-ref", "HEAD"))
	if head != "master" {
		t.Fatalf("HEAD=%q log=\n%s", head, buf.String())
	}
	if strings.Contains(gitOut(t, work, "tag", "-l"), "v9.9.9") {
		t.Fatal("fetch must not import remote tags")
	}
}

func TestCreateBranchFromRemoteWhenLocalIsOtherBranch(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not in PATH")
	}
	root := t.TempDir()
	src := filepath.Join(root, "src")
	origin := filepath.Join(root, "origin.git")
	work := filepath.Join(root, "work")
	mustGit(t, "", "init", "-b", "main", src)
	mustWrite(t, filepath.Join(src, "README.md"), "main\n")
	mustGit(t, src, "add", "README.md")
	mustGit(t, src, "commit", "-m", "main")
	mustGit(t, src, "checkout", "-B", "master")
	mustWrite(t, filepath.Join(src, "from-master.txt"), "latest\n")
	mustGit(t, src, "add", "from-master.txt")
	mustGit(t, src, "commit", "-m", "master tip")
	mustGit(t, src, "checkout", "main")
	mustGit(t, "", "clone", "--bare", src, origin)
	mustGit(t, "", "clone", "--depth", "1", "--branch", "main", "--no-tags", origin, work)

	var buf bytes.Buffer
	if err := CreateBranch(context.Background(), work, "master", "rel-1", false, Options{Depth: 1}, &buf); err != nil {
		t.Fatalf("CreateBranch: %v\n%s", err, buf.String())
	}
	head := strings.TrimSpace(gitOut(t, work, "rev-parse", "--abbrev-ref", "HEAD"))
	if head != "rel-1" {
		t.Fatalf("HEAD=%q log=\n%s", head, buf.String())
	}
	if _, err := os.Stat(filepath.Join(work, "from-master.txt")); err != nil {
		t.Fatalf("expected latest master file: %v\n%s", err, buf.String())
	}
	if !strings.Contains(buf.String(), "不是源分支") {
		t.Fatalf("should log local/source mismatch\n%s", buf.String())
	}
}

func TestCreateBranchForceOverwritesExistingRemote(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not in PATH")
	}
	root := t.TempDir()
	src := filepath.Join(root, "src")
	origin := filepath.Join(root, "origin.git")
	work := filepath.Join(root, "work")
	mustGit(t, "", "init", "-b", "main", src)
	mustWrite(t, filepath.Join(src, "README.md"), "old\n")
	mustGit(t, src, "add", "README.md")
	mustGit(t, src, "commit", "-m", "old")
	mustGit(t, src, "checkout", "-B", "rel-1")
	mustWrite(t, filepath.Join(src, "old-rel.txt"), "keep\n")
	mustGit(t, src, "add", "old-rel.txt")
	mustGit(t, src, "commit", "-m", "rel only")
	mustGit(t, src, "checkout", "main")
	mustGit(t, src, "checkout", "-B", "master")
	mustWrite(t, filepath.Join(src, "README.md"), "latest\n")
	mustGit(t, src, "add", "README.md")
	mustGit(t, src, "commit", "-m", "master tip")
	mustGit(t, src, "checkout", "main")
	mustGit(t, "", "clone", "--bare", src, origin)
	mustGit(t, "", "clone", "--depth", "1", "--branch", "main", "--no-tags", origin, work)

	var buf bytes.Buffer
	if err := CreateBranch(context.Background(), work, "master", "rel-1", false, Options{Depth: 1}, &buf); err == nil {
		t.Fatalf("expected non-fast-forward failure\n%s", buf.String())
	}
	buf.Reset()
	if err := CreateBranch(context.Background(), work, "master", "rel-1", true, Options{Depth: 1}, &buf); err != nil {
		t.Fatalf("force CreateBranch: %v\n%s", err, buf.String())
	}
	got := gitOut(t, origin, "show", "rel-1:README.md")
	if !strings.Contains(got, "latest") {
		t.Fatalf("remote not overwritten: %q\n%s", got, buf.String())
	}
}

func TestCommitPushPomAndSkipWhenUnchanged(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not in PATH")
	}
	root := t.TempDir()
	src := filepath.Join(root, "src")
	origin := filepath.Join(root, "origin.git")
	work := filepath.Join(root, "work")
	mustGit(t, "", "init", "-b", "main", src)
	mustWrite(t, filepath.Join(src, "pom.xml"), "<project><version>1.0.0</version></project>\n")
	mustGit(t, src, "add", "pom.xml")
	mustGit(t, src, "commit", "-m", "init")
	mustGit(t, "", "clone", "--bare", src, origin)
	mustGit(t, "", "clone", origin, work)
	mustWrite(t, filepath.Join(work, "pom.xml"), "<project><version>2.0.0</version></project>\n")

	var buf bytes.Buffer
	if err := CommitPush(context.Background(), work, "chore: set pom version to 2.0.0", []string{"pom.xml"}, Options{}, &buf); err != nil {
		t.Fatalf("CommitPush: %v\n%s", err, buf.String())
	}
	if !strings.Contains(gitOut(t, origin, "log", "-1", "--pretty=%s"), "set pom version to 2.0.0") {
		t.Fatalf("origin missing commit\n%s", buf.String())
	}
	buf.Reset()
	if err := CommitPush(context.Background(), work, "chore: set pom version to 2.0.0", []string{"pom.xml"}, Options{}, &buf); err != nil {
		t.Fatalf("second CommitPush: %v\n%s", err, buf.String())
	}
	if !strings.Contains(buf.String(), "无变更") {
		t.Fatalf("expected skip when unchanged\n%s", buf.String())
	}
}

func TestLastUserCommitSkipsPack(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not in PATH")
	}
	dir := t.TempDir()
	mustGit(t, "", "init", "-b", "main", dir)
	mustWrite(t, filepath.Join(dir, "a.txt"), "one\n")
	mustGit(t, dir, "add", "a.txt")
	mustGit(t, dir, "commit", "-m", "fix: user change")
	mustWrite(t, filepath.Join(dir, "b.txt"), "bot\n")
	mustGit(t, dir, "add", "b.txt")
	cmd := exec.Command("git", "-c", "user.name=auto-pack", "-c", "user.email=auto-pack@local", "-c", "commit.gpgsign=false", "commit", "-m", "chore: tag")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_TERMINAL_PROMPT=0")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("bot commit: %v\n%s", err, out)
	}
	got := LastUserCommit(context.Background(), dir, Options{})
	if !strings.Contains(got, "fix: user change") || strings.Contains(got, "chore: tag") {
		t.Fatal(got)
	}
}

func mustWrite(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func mustGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	if err := runGit(t, dir, args...); err != nil {
		t.Fatalf("git %s: %v", strings.Join(args, " "), err)
	}
}

func runGit(t *testing.T, dir string, args ...string) error {
	t.Helper()
	cmd := exec.Command("git", append([]string{
		"-c", "user.name=test",
		"-c", "user.email=test@example.com",
		"-c", "commit.gpgsign=false",
		"-c", "init.defaultBranch=main",
	}, args...)...)
	if dir != "" {
		cmd.Dir = dir
	}
	cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_TERMINAL_PROMPT=0")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Logf("git %s\n%s", strings.Join(args, " "), out)
	}
	return err
}

func gitOut(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return string(out)
}
