// Package npm 跑 npm / pnpm。无 HTTP、无权限表。
package npm

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"

	"auto-pack/server/internal/host/bash"
)

// Run executes an npm/pnpm command. Empty command becomes "npm run build"
// (or pnpm if pnpm-lock.yaml exists).
func Run(ctx context.Context, dir, command string, env []string, w io.Writer) error {
	command = strings.TrimSpace(command)
	if command == "" {
		command = defaultBuild(dir)
	}
	command, err := ensurePnpm(ctx, command, env, w)
	if err != nil {
		return err
	}
	return bash.Run(ctx, dir, command, env, w)
}

func defaultBuild(dir string) string {
	if _, err := os.Stat(filepath.Join(dir, "pnpm-lock.yaml")); err == nil {
		return "pnpm run build"
	}
	if _, err := os.Stat(filepath.Join(dir, "yarn.lock")); err == nil {
		return "yarn build"
	}
	return "npm run build"
}
