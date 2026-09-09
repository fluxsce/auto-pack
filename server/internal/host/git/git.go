// Package git 跑 clone / fetch / tag。无 HTTP、无权限表。
package git

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Auth is workspace git env (HTTPS token / SSH key / commit identity).
type Auth struct {
	AuthType    string
	Token       string
	Username    string
	Password    string
	KeyPath     string
	Passphrase  string
	GitPath     string
	CommitName  string
	CommitEmail string
}

// Options controls clone/fetch depth and credentials.
type Options struct {
	Depth int
	Env   []string
	Auth  Auth
}

func (o Options) bin() string {
	if p := strings.TrimSpace(o.Auth.GitPath); p != "" {
		return p
	}
	return "git"
}

func (o Options) extraArgs() []string {
	var args []string
	if name := strings.TrimSpace(o.Auth.CommitName); name != "" {
		args = append(args, "-c", "user.name="+name)
	}
	if email := strings.TrimSpace(o.Auth.CommitEmail); email != "" {
		args = append(args, "-c", "user.email="+email)
	}
	user, pass := o.Auth.userPass()
	if o.Auth.useHTTPS(user, pass) {
		args = append(args, "-c", "credential.helper=", "-c", "credential.helper="+askHelper(user, pass))
	}
	return args
}

func (a Auth) useHTTPS(user, pass string) bool {
	if a.AuthType == "ssh" || a.AuthType == "none" {
		return false
	}
	return user != "" || pass != ""
}

func (a Auth) userPass() (string, string) {
	if tok := strings.TrimSpace(a.Token); tok != "" {
		user := strings.TrimSpace(a.Username)
		if user == "" {
			user = "x-access-token"
		}
		return user, tok
	}
	return strings.TrimSpace(a.Username), strings.TrimSpace(a.Password)
}

func (o Options) extraEnv() []string {
	env := append([]string{}, o.Env...)
	user, pass := o.Auth.userPass()
	if o.Auth.useHTTPS(user, pass) {
		return env
	}
	if key := strings.TrimSpace(o.Auth.KeyPath); key != "" {
		env = append(env, "GIT_SSH_COMMAND=ssh -i "+quotePath(key)+" -o IdentitiesOnly=yes -o StrictHostKeyChecking=accept-new")
	}
	return env
}

func run(ctx context.Context, dir string, opt Options, w io.Writer, args ...string) error {
	all := append(opt.extraArgs(), args...)
	cmd := exec.CommandContext(ctx, opt.bin(), all...)
	if dir != "" {
		cmd.Dir = dir
	}
	if extra := opt.extraEnv(); len(extra) > 0 {
		cmd.Env = append(os.Environ(), extra...)
	}
	if w != nil {
		cmd.Stdout = w
		cmd.Stderr = w
	}
	return cmd.Run()
}

// Clone clones url into dest. Only the requested branch is fetched; tags stay on the remote.
func Clone(ctx context.Context, url, dest, branch string, opt Options, w io.Writer) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	args := []string{"clone", "--no-tags"}
	if opt.Depth > 0 {
		args = append(args, "--depth", fmt.Sprintf("%d", opt.Depth))
	}
	if b := strings.TrimSpace(branch); b != "" {
		args = append(args, "--branch", b, "--single-branch")
	}
	args = append(args, url, dest)
	return run(ctx, "", opt, w, args...)
}

// FetchPull updates the work tree to origin's tip of branch.
// One fetch of that ref (no --all, no tags, no second pull), then reset the local branch.
func FetchPull(ctx context.Context, dir, branch string, opt Options, w io.Writer) error {
	branch = strings.TrimSpace(branch)
	if branch == "" {
		if err := run(ctx, dir, opt, w, fetchArgs(opt, "fetch", "--no-tags", "origin")...); err != nil {
			return err
		}
		return run(ctx, dir, opt, w, "merge", "--ff-only", "FETCH_HEAD")
	}
	if err := fetchOriginBranch(ctx, dir, opt, w, branch); err != nil {
		return err
	}
	return checkoutRemoteBranch(ctx, dir, opt, w, branch)
}

func fetchArgs(opt Options, args ...string) []string {
	if opt.Depth > 0 {
		return append(args, "--depth", fmt.Sprintf("%d", opt.Depth))
	}
	return args
}

func fetchOriginBranch(ctx context.Context, dir string, opt Options, w io.Writer, branch string) error {
	refSpec := "+refs/heads/" + branch + ":refs/remotes/origin/" + branch
	if err := run(ctx, dir, opt, w, fetchArgs(opt, "fetch", "--no-tags", "origin", refSpec)...); err == nil {
		return nil
	}
	return run(ctx, dir, opt, w, fetchArgs(opt, "fetch", "--no-tags", "origin", branch)...)
}

func checkoutRemoteBranch(ctx context.Context, dir string, opt Options, w io.Writer, branch string) error {
	remote := "origin/" + branch
	if err := run(ctx, dir, opt, w, "checkout", "-f", "-B", branch, remote); err != nil {
		return fmt.Errorf("checkout %s from %s: %w", branch, remote, err)
	}
	return nil
}

// CreateBranch fetches the latest origin/from, creates name from that tip, and pushes it.
// Local HEAD may be on another branch; the new branch is never created from a stale checkout.
// force overwrites origin/name when it already exists.
func CreateBranch(ctx context.Context, dir, from, name string, force bool, opt Options, w io.Writer) error {
	from = strings.TrimSpace(from)
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("branch name required")
	}
	if from == "" {
		return fmt.Errorf("source branch required")
	}
	remote := "origin/" + from
	head := headBranch(ctx, dir, opt)
	if head != "" && head != from {
		note(w, "本地当前在 %s，不是源分支 %s，改为从远程 %s 最新提交创建", head, from, remote)
	} else if head == from {
		note(w, "本地已在源分支 %s，仍以远程 %s 最新提交为准", from, remote)
	} else {
		note(w, "从远程 %s 最新提交创建 %s", remote, name)
	}
	if err := fetchOriginBranch(ctx, dir, opt, w, from); err != nil {
		return err
	}
	if tip, err := runOut(ctx, dir, opt, "rev-parse", "--short", remote); err == nil && tip != "" {
		note(w, "远程 %s 最新 %s", remote, tip)
	}
	if err := run(ctx, dir, opt, w, "checkout", "-f", "-B", name, remote); err != nil {
		return fmt.Errorf("checkout %s from %s: %w", name, remote, err)
	}
	if force {
		note(w, "覆盖推送 origin/%s（远程已存在则重置为源分支最新）", name)
		return run(ctx, dir, opt, w, "push", "-u", "--force", "origin", name)
	}
	note(w, "推送 origin/%s（远程已存在且不能快进则失败）", name)
	return run(ctx, dir, opt, w, "push", "-u", "origin", name)
}

// ShortHead returns rev-parse --short HEAD, or empty when the work tree has no commit.
func ShortHead(ctx context.Context, dir string, opt Options) string {
	out, err := runOut(ctx, dir, opt, "rev-parse", "--short", "HEAD")
	if err != nil {
		return ""
	}
	return out
}

func headBranch(ctx context.Context, dir string, opt Options) string {
	out, err := runOut(ctx, dir, opt, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil || out == "" || out == "HEAD" {
		return ""
	}
	return out
}

func runOut(ctx context.Context, dir string, opt Options, args ...string) (string, error) {
	var buf bytes.Buffer
	err := run(ctx, dir, opt, &buf, args...)
	return strings.TrimSpace(buf.String()), err
}

func note(w io.Writer, format string, args ...any) {
	if w == nil {
		return
	}
	_, _ = fmt.Fprintf(w, format+"\n", args...)
}

// CommitPush stages files, commits when there is a diff, and pushes HEAD to origin.
func CommitPush(ctx context.Context, dir, message string, files []string, opt Options, w io.Writer) error {
	if len(files) == 0 {
		return fmt.Errorf("no files to commit")
	}
	message = strings.TrimSpace(message)
	if message == "" {
		return fmt.Errorf("commit message required")
	}
	opt = withCommitIdent(opt)
	add := append([]string{"add", "--"}, files...)
	if err := run(ctx, dir, opt, w, add...); err != nil {
		return err
	}
	if err := run(ctx, dir, opt, io.Discard, "diff", "--cached", "--quiet"); err == nil {
		note(w, "工作区无变更，跳过提交")
		return nil
	}
	if err := run(ctx, dir, opt, w, "-c", "commit.gpgsign=false", "commit", "-m", message); err != nil {
		return err
	}
	return run(ctx, dir, opt, w, "push", "origin", "HEAD")
}

func withCommitIdent(opt Options) Options {
	if strings.TrimSpace(opt.Auth.CommitName) == "" {
		opt.Auth.CommitName = "auto-pack"
	}
	if strings.TrimSpace(opt.Auth.CommitEmail) == "" {
		opt.Auth.CommitEmail = "auto-pack@local"
	}
	return opt
}

// Tag creates an annotated tag and optionally pushes.
func Tag(ctx context.Context, dir, name, message string, push bool, opt Options, w io.Writer) error {
	if err := run(ctx, dir, opt, w, "tag", "-a", name, "-m", message); err != nil {
		return err
	}
	if push {
		return run(ctx, dir, opt, w, "push", "origin", name)
	}
	return nil
}

// IsRepo reports whether dir looks like a git repo.
func IsRepo(dir string) bool {
	st, err := os.Stat(filepath.Join(dir, ".git"))
	return err == nil && st.IsDir()
}

// MapAuth reads a decrypted git env config map.
func MapAuth(m map[string]any) Auth {
	if m == nil {
		return Auth{}
	}
	return Auth{
		AuthType:    asString(m["auth_type"]),
		Token:       asString(m["auth_token"]),
		Username:    asString(m["auth_username"]),
		Password:    asString(m["auth_password"]),
		KeyPath:     asString(m["private_key_path"]),
		Passphrase:  asString(m["private_key_passphrase"]),
		GitPath:     asString(m["git_path"]),
		CommitName:  asString(m["username"]),
		CommitEmail: asString(m["email"]),
	}
}

func askHelper(user, pass string) string {
	return fmt.Sprintf("!f() { echo username=%s; echo password=%s; }; f", sanitizeCred(user), sanitizeCred(pass))
}

func sanitizeCred(s string) string {
	s = strings.ReplaceAll(s, "\r", "")
	s = strings.ReplaceAll(s, "\n", "")
	return s
}

func quotePath(p string) string {
	return `"` + strings.ReplaceAll(p, `"`, `\"`) + `"`
}

func asString(v any) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return strings.TrimSpace(s)
	}
	return strings.TrimSpace(fmt.Sprint(v))
}
