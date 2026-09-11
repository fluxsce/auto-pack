package release

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"auto-pack/server/internal/project"
	"auto-pack/server/internal/release/publish"
	"auto-pack/server/internal/release/record"
)

func TestReleaserKeepsNonMavenWithoutVersionFiles(t *testing.T) {
	dir := t.TempDir()
	for _, typ := range []string{"go", "custom"} {
		out := (&Releaser{}).Run(context.Background(), project.Project{ProjectName: typ + "-cli", ProjectType: typ}, Request{
			WorkDir: dir, ReleaseBranch: "release/V1", Tag: "V1", UpdatePomVersion: true,
		})
		if out.Status != record.ResultOK {
			t.Fatalf("%s: %+v", typ, out)
		}
		if !strings.Contains(strings.Join(out.Steps, "\n"), "仍会打 Tag") {
			t.Fatalf("%s: expected tag-only step, got %+v", typ, out)
		}
		if _, err := os.Stat(filepath.Join(dir, "src", "main", "resources", "Version.txt")); err == nil {
			t.Fatalf("%s: must not invent Maven Version.txt", typ)
		}
	}
}

func TestReleaserWritesMavenVersionWithoutGit(t *testing.T) {
	dir := t.TempDir()
	out := (&Releaser{}).Run(context.Background(), project.Project{ProjectName: "svc", ProjectType: "maven"}, Request{
		WorkDir: dir, ReleaseBranch: "release/V1", Tag: "V1.0.0-P01",
	})
	if out.Status != record.ResultOK {
		t.Fatalf("%+v", out)
	}
	if _, err := os.Stat(filepath.Join(dir, "src", "main", "resources", "Version.txt")); err != nil {
		t.Fatalf("Version.txt: %v", err)
	}
}

func TestReleaserRejectsEmptyWorkDir(t *testing.T) {
	out := (&Releaser{}).Run(context.Background(), project.Project{ProjectName: "app", ProjectType: "maven"}, Request{
		ReleaseBranch: "release/V1", Tag: "V1",
	})
	if out.Status != record.ResultFailed || out.Message == "" {
		t.Fatalf("%+v", out)
	}
}

func TestReleaserTagsGoProjectWithoutPom(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not in PATH")
	}
	root := t.TempDir()
	src := filepath.Join(root, "src")
	origin := filepath.Join(root, "origin.git")
	work := filepath.Join(root, "work")
	mustGit(t, "", "init", "-b", "main", src)
	mustWrite(t, filepath.Join(src, "main.go"), "package main\n")
	mustGit(t, src, "add", "main.go")
	mustGit(t, src, "commit", "-m", "init")
	mustGit(t, src, "checkout", "-B", "release/V9.0.0-P01")
	mustGit(t, "", "clone", "--bare", src, origin)
	mustGit(t, "", "clone", origin, work)

	rel := &Releaser{Git: &publish.GitCommitter{}}
	out := rel.Run(context.Background(), project.Project{
		ProjectName: "cli", ProjectType: "go", RepoURL: origin,
	}, Request{
		WorkDir: work, ReleaseBranch: "release/V9.0.0-P01", Tag: "V9.0.0-P01-01",
		AllowRepeat: true, UpdatePomVersion: true,
	})
	if out.Status != record.ResultOK {
		t.Fatalf("%+v", out)
	}
	if !strings.Contains(out.Message, "git tag V9.0.0-P01-01") {
		t.Fatalf("message: %+v", out)
	}
	tags := gitOut(t, origin, "tag", "-l")
	if !strings.Contains(tags, "V9.0.0-P01-01") {
		t.Fatalf("origin tags:\n%s\nsteps:\n%s", tags, strings.Join(out.Steps, "\n"))
	}
	if _, err := os.Stat(filepath.Join(work, "src", "main", "resources", "Version.txt")); err == nil {
		t.Fatal("go project must not create Maven Version.txt")
	}
}

func mustWrite(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func mustGit(t *testing.T, dir string, args ...string) {
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
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
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
