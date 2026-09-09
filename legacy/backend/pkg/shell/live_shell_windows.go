//go:build windows

package shell

import (
	"context"
	"os/exec"
)

func shellCommandForLive(ctx context.Context, script string) *exec.Cmd {
	return shellCommand(ctx, script)
}
