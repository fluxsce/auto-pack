package packrun

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"auto-pack/backend/pkg/shell"
)

type buildLogger struct {
	f *os.File
}

func newBuildLogger(f *os.File) *buildLogger {
	return &buildLogger{f: f}
}

func (l *buildLogger) blank() {
	_, _ = fmt.Fprintln(l.f)
}

func (l *buildLogger) line(format string, args ...any) {
	_, _ = fmt.Fprintf(l.f, format+"\n", args...)
}

func (l *buildLogger) step(format string, args ...any) {
	l.line(format, args...)
}

func (l *buildLogger) section(title string) {
	l.blank()
	l.line("[%s]", title)
}

func (l *buildLogger) cmd(name string, args []string) {
	l.line(" > %s %s", name, strings.Join(args, " "))
}

func (l *buildLogger) cmdScript(script string) {
	l.line(" > %s %s", shell.ShellLabel(), quoteScriptForLog(script))
}

func (l *buildLogger) emitOutput(res shell.Result) {
	if out := strings.TrimRight(res.Stdout, "\r\n"); out != "" {
		_, _ = fmt.Fprintln(l.f, out)
	}
	if errOut := strings.TrimRight(res.Stderr, "\r\n"); errOut != "" {
		_, _ = fmt.Fprintln(l.f, errOut)
	}
}

func (l *buildLogger) runGit(
	ctx context.Context,
	dir string,
	env []string,
	args ...string,
) (shell.Result, error) {
	l.cmd("git", args)
	res, err := shell.RunArgs(ctx, "git", args, shell.Options{
		Dir:              dir,
		Env:              env,
		AllowNonZeroExit: true,
	})
	l.emitOutput(res)
	if err != nil {
		return res, err
	}
	if res.ExitCode != 0 {
		return res, fmt.Errorf("git %s: exit %d", strings.Join(args, " "), res.ExitCode)
	}
	return res, nil
}

func (l *buildLogger) runShell(
	ctx context.Context,
	dir string,
	env []string,
	script string,
) (shell.Result, error) {
	l.cmdScript(script)
	res, err := shell.Run(ctx, script, shell.Options{
		Dir:              dir,
		Env:              env,
		AllowNonZeroExit: true,
	})
	l.emitOutput(res)
	return res, err
}

func (l *buildLogger) header(projectName string, buildNum int, workDir string) {
	host, _ := os.Hostname()
	if host == "" {
		host = "localhost"
	}
	userName := "SYSTEM"
	if u, err := user.Current(); err == nil && strings.TrimSpace(u.Username) != "" {
		userName = u.Username
	}
	l.line("Started by Auto Pack")
	l.line("Building %s #%d", projectName, buildNum)
	l.line("Running as %s on %s (%s)", userName, host, runtime.GOOS)
	l.blank()
	l.step("Building in workspace %s", filepath.ToSlash(workDir))
}

func (l *buildLogger) finished(status string, message string) {
	l.blank()
	switch status {
	case "success":
		l.line("Finished: SUCCESS")
	case "failed":
		l.line("Finished: FAILURE")
		if message != "" {
			l.line("ERROR: %s", message)
		}
	default:
		l.line("Finished: %s", strings.ToUpper(status))
	}
}

func sanitizeURLForLog(raw string) string {
	raw = strings.TrimSpace(raw)
	u, err := url.Parse(raw)
	if err != nil || u.User == nil {
		return raw
	}
	host := u.Host
	if u.User != nil {
		host = "****@" + host
	}
	out := u.Scheme + "://" + host + u.EscapedPath()
	if u.RawQuery != "" {
		out += "?" + u.RawQuery
	}
	return out
}

func quoteScriptForLog(script string) string {
	script = strings.TrimSpace(script)
	if strings.ContainsAny(script, " \t\n\"") {
		return fmt.Sprintf("%q", script)
	}
	return script
}

func logTimestamp() string {
	return time.Now().Format("2006-01-02 15:04:05")
}

func (l *buildLogger) logGitCheckoutSummary(
	ctx context.Context,
	dir string,
	env []string,
	branch string,
) {
	branch = strings.TrimSpace(branch)
	if branch == "" {
		branch = "main"
	}
	revRes, err := shell.RunArgs(ctx, "git", []string{"rev-parse", "HEAD"}, shell.Options{
		Dir:              dir,
		Env:              env,
		AllowNonZeroExit: true,
	})
	if err != nil || revRes.ExitCode != 0 {
		return
	}
	rev := strings.TrimSpace(revRes.Stdout)
	if rev == "" {
		return
	}
	l.step("Checking out Revision %s (refs/remotes/origin/%s)", rev, branch)

	msgRes, err := shell.RunArgs(ctx, "git", []string{"log", "-1", "--pretty=format:%s"}, shell.Options{
		Dir:              dir,
		Env:              env,
		AllowNonZeroExit: true,
	})
	if err != nil || msgRes.ExitCode != 0 {
		return
	}
	msg := strings.TrimSpace(msgRes.Stdout)
	if msg == "" {
		return
	}
	l.step("Commit message: %q", msg)
}
