//go:build !windows

package shell

import (
	"context"
	"os/exec"
)

// shellCommandForLive prefers stdbuf line-buffering so CI logs update while tools run.
func shellCommandForLive(ctx context.Context, script string) *exec.Cmd {
	if _, err := exec.LookPath("stdbuf"); err == nil {
		return exec.CommandContext(ctx, "stdbuf", "-oL", "-eL", ShellExecutable(), "-c", script)
	}
	return shellCommand(ctx, script)
}
