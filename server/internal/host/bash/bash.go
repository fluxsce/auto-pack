// Package bash 跑本机命令。无 HTTP、无权限表。
package bash

import (
	"context"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// Result is command output.
type Result struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

// Run executes command in dir, streaming combined output to w.
func Run(ctx context.Context, dir, command string, env []string, w io.Writer) error {
	command = strings.TrimSpace(command)
	if command == "" {
		return nil
	}
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(ctx, "cmd", "/c", command)
	} else {
		cmd = exec.CommandContext(ctx, "bash", "-lc", command)
	}
	if dir != "" {
		cmd.Dir = dir
	}
	if len(env) > 0 {
		cmd.Env = append(os.Environ(), env...)
	}
	out := discardIfNil(w)
	cmd.Stdout = out
	cmd.Stderr = out
	return cmd.Run()
}

func discardIfNil(w io.Writer) io.Writer {
	if w == nil {
		return io.Discard
	}
	return w
}
