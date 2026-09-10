package git

import (
	"strings"
	"testing"
)

func TestPickUserCommit(t *testing.T) {
	raw := "auto-pack\tauto-pack@local\tabc123 auto-pack 2026-09-10 20:00 chore: tag\n" +
		"张三\tzhang@example.com\tdef456 张三 2026-09-10 18:20 fix: timeout\n"
	got := pickUserCommit(raw)
	if got != "def456 张三 2026-09-10 18:20 fix: timeout" {
		t.Fatal(got)
	}
	if pickUserCommit("auto-pack\tauto-pack@local\tabc123 auto-pack 2026-09-10 20:00 chore: tag\n") != "abc123 auto-pack 2026-09-10 20:00 chore: tag" {
		t.Fatal("fallback")
	}
	if pickUserCommit("") != "" {
		t.Fatal("empty")
	}
}

func TestLastGitLine(t *testing.T) {
	if lastGitLine("") != "" {
		t.Fatal("empty")
	}
	got := lastGitLine("From origin\nerror: pathspec 'origin/develop' did not match any file(s) known to git.\n")
	if got != "error: pathspec 'origin/develop' did not match any file(s) known to git." {
		t.Fatal(got)
	}
}

func TestMapAuthHTTPS(t *testing.T) {
	a := MapAuth(map[string]any{
		"auth_type":  "https",
		"auth_token": "tok",
		"git_path":   "/usr/bin/git",
		"username":   "bot",
		"email":      "bot@example.com",
	})
	if a.Token != "tok" || a.GitPath != "/usr/bin/git" || a.CommitName != "bot" {
		t.Fatalf("%+v", a)
	}
	user, pass := a.userPass()
	if user != "x-access-token" || pass != "tok" {
		t.Fatalf("userPass %s %s", user, pass)
	}
	opt := Options{Auth: a}
	args := strings.Join(opt.extraArgs(), " ")
	if !strings.Contains(args, "credential.helper=") || !strings.Contains(args, "user.name=bot") {
		t.Fatal(args)
	}
	if opt.bin() != "/usr/bin/git" {
		t.Fatal(opt.bin())
	}
}

func TestHTTPSIgnoresSSHKey(t *testing.T) {
	opt := Options{Auth: MapAuth(map[string]any{
		"auth_type":        "https",
		"auth_token":       "tok",
		"private_key_path": "/tmp/id",
	})}
	if strings.Contains(strings.Join(opt.extraEnv(), "\n"), "GIT_SSH_COMMAND") {
		t.Fatal(opt.extraEnv())
	}
}

func TestMapAuthSSHSkipsHelper(t *testing.T) {
	opt := Options{Auth: MapAuth(map[string]any{
		"auth_type":        "ssh",
		"private_key_path": "/tmp/id",
	})}
	if strings.Contains(strings.Join(opt.extraArgs(), " "), "credential.helper") {
		t.Fatal(opt.extraArgs())
	}
	env := strings.Join(opt.extraEnv(), "\n")
	if !strings.Contains(env, "GIT_SSH_COMMAND=") || !strings.Contains(env, "/tmp/id") {
		t.Fatal(env)
	}
}
