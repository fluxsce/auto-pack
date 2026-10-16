//go:build unix

package serverupdate

import (
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
)

func schedule(root string) error {
	if err := os.MkdirAll(filepath.Join(root, "data"), 0o755); err != nil {
		return err
	}
	helper := filepath.Join(root, "data", "server-update.sh")
	body := unixScript(root, binaryPath(root), stagedBinary(root))
	if err := os.WriteFile(helper, []byte(body), 0o755); err != nil {
		return err
	}
	cmd := exec.Command("/bin/sh", helper)
	cmd.Dir = root
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	return cmd.Start()
}
