package git

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// Result is one git command outcome. ExitCode is set when the process ran.
type Result struct {
	ExitCode int
	Stdout   string
	Stderr   string
}

// Exec runs git args and always returns the process result (including non-zero exit).
func Exec(ctx context.Context, dir string, opt Options, args ...string) (Result, error) {
	all := append(opt.extraArgs(), args...)
	cmd := exec.CommandContext(ctx, opt.bin(), all...)
	if dir != "" {
		cmd.Dir = dir
	}
	if extra := opt.extraEnv(); len(extra) > 0 {
		cmd.Env = append(os.Environ(), extra...)
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	res := Result{Stdout: stdout.String(), Stderr: stderr.String()}
	if cmd.ProcessState != nil {
		res.ExitCode = cmd.ProcessState.ExitCode()
	}
	if err != nil && res.ExitCode == 0 {
		return res, err
	}
	return res, nil
}

// ExecOK runs git and fails when the exit code is not zero.
func ExecOK(ctx context.Context, dir string, opt Options, args ...string) (Result, error) {
	res, err := Exec(ctx, dir, opt, args...)
	if err != nil {
		return res, err
	}
	if res.ExitCode != 0 {
		return res, fmtGitErr(args, res)
	}
	return res, nil
}

func fmtGitErr(args []string, res Result) error {
	detail := strings.TrimSpace(res.Stderr)
	if detail == "" {
		detail = strings.TrimSpace(res.Stdout)
	}
	if detail == "" {
		detail = fmt.Sprintf("exit %d", res.ExitCode)
	}
	return &cmdError{msg: "git " + strings.Join(args, " ") + ": " + detail, code: res.ExitCode}
}

type cmdError struct {
	msg  string
	code int
}

func (e *cmdError) Error() string { return e.msg }
