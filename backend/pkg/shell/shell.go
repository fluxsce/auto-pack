package shell

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"auto-pack/backend/pkg/redact"
)

// Result holds captured output and process exit status.
type Result struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

// Options configures a single command invocation.
type Options struct {
	// Dir is the working directory. Empty uses the current process directory.
	Dir string
	// Env entries are appended to the process environment (KEY=value).
	Env []string
	// Stdin is optional standard input for the child process.
	Stdin io.Reader
	// AllowNonZeroExit when true returns (Result, nil) even if the process exits non-zero.
	AllowNonZeroExit bool
}

// ExitError is returned when a command finishes with a non-zero exit code.
type ExitError struct {
	Name   string
	Args   []string
	Script string
	Result Result
}

func (e *ExitError) Error() string {
	label := commandLabel(e.Name, e.Args, e.Script)
	return fmt.Sprintf(
		"%s: exit %d (stderr: %s)",
		label,
		e.Result.ExitCode,
		redact.Redact(trimForError(e.Result.Stderr)),
	)
}

// Run executes script through the platform shell (cmd /C on Windows, sh -c elsewhere).
func Run(ctx context.Context, script string, opts Options) (Result, error) {
	script = strings.TrimSpace(script)
	if script == "" {
		return Result{}, fmt.Errorf("shell: empty script")
	}
	cmd := shellCommand(ctx, script)
	return run(ctx, cmd, opts, "", nil, script)
}

// RunArgs runs an executable directly without a shell wrapper.
func RunArgs(ctx context.Context, name string, args []string, opts Options) (Result, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return Result{}, fmt.Errorf("shell: empty command name")
	}
	cmd := exec.CommandContext(ctx, name, args...)
	return run(ctx, cmd, opts, name, args, "")
}

func run(ctx context.Context, cmd *exec.Cmd, opts Options, name string, args []string, script string) (Result, error) {
	if opts.Dir != "" {
		cmd.Dir = opts.Dir
	}
	if len(opts.Env) > 0 {
		cmd.Env = append(os.Environ(), opts.Env...)
	}
	if opts.Stdin != nil {
		cmd.Stdin = opts.Stdin
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	res := Result{
		Stdout: DecodeProcessOutput(stdout.Bytes()),
		Stderr: DecodeProcessOutput(stderr.Bytes()),
	}
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			res.ExitCode = exitErr.ExitCode()
		} else if ctx.Err() != nil {
			return res, fmt.Errorf("%s: %w", commandLabel(name, args, script), ctx.Err())
		} else {
			return res, fmt.Errorf("%s: %w", commandLabel(name, args, script), err)
		}
	}

	if res.ExitCode != 0 && !opts.AllowNonZeroExit {
		return res, &ExitError{
			Name:   name,
			Args:   args,
			Script: script,
			Result: res,
		}
	}
	return res, nil
}

func commandLabel(name string, args []string, script string) string {
	if script != "" {
		return redact.Redact(shellLabel() + " " + quoteScript(script))
	}
	if len(args) == 0 {
		return name
	}
	safe := redact.CommandArgs(args)
	return name + " " + strings.Join(safe, " ")
}

func quoteScript(script string) string {
	if strings.ContainsAny(script, " \t\n\"") {
		return fmt.Sprintf("%q", script)
	}
	return script
}

func trimForError(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > 200 {
		return s[:200] + "..."
	}
	if s == "" {
		return "(empty)"
	}
	return s
}
