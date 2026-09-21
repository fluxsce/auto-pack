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
	name, args := shellArgs(command)
	cmd := exec.CommandContext(ctx, name, args...)
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

// shellArgs 选择非登录壳。Linux 上 bash -l 会再读 /etc/profile，把已注入的 JAVA_HOME 盖成机器默认 JDK。
func shellArgs(command string) (name string, args []string) {
	if runtime.GOOS == "windows" {
		return "cmd", []string{"/c", command}
	}
	return "bash", []string{"--noprofile", "--norc", "-c", command}
}
