//go:build windows

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
	helper := filepath.Join(root, "data", "server-update.cmd")
	body := windowsScript(root, binaryPath(root), stagedBinary(root))
	if err := os.WriteFile(helper, []byte(body), 0o644); err != nil {
		return err
	}
	cmd := exec.Command("cmd", "/C", helper)
	cmd.Dir = root
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: 0x00000008 | 0x00000200} // DETACHED_PROCESS | CREATE_NEW_PROCESS_GROUP
	cmd.Stdout = nil
	cmd.Stderr = nil
	return cmd.Start()
}
