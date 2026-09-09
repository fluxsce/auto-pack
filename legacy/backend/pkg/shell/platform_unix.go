//go:build !windows

package shell

import (
	"context"
	"os/exec"
)

// ShellExecutable returns the shell binary used by Run.
func ShellExecutable() string { return "sh" }

func shellCommand(ctx context.Context, script string) *exec.Cmd {
	return exec.CommandContext(ctx, ShellExecutable(), "-c", script)
}

// ShellLabel describes how Run invokes the platform shell.
func ShellLabel() string { return "sh -c" }

func shellLabel() string { return ShellLabel() }
