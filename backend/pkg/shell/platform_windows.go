//go:build windows

package shell

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
)

// ShellExecutable returns the shell binary used by Run (typically cmd.exe on Windows).
func ShellExecutable() string { return windowsShellExe() }

func shellCommand(ctx context.Context, script string) *exec.Cmd {
	return exec.CommandContext(ctx, ShellExecutable(), "/C", script)
}

func windowsShellExe() string {
	if comspec := os.Getenv("COMSPEC"); comspec != "" {
		return comspec
	}
	for _, root := range []string{
		os.Getenv("SystemRoot"),
		os.Getenv("SYSTEMROOT"),
		`C:\Windows`,
	} {
		if root == "" {
			continue
		}
		p := filepath.Join(root, "System32", "cmd.exe")
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return "cmd"
}

// ShellLabel describes how Run invokes the platform shell.
func ShellLabel() string { return "cmd /C" }

func shellLabel() string { return ShellLabel() }
