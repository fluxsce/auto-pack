package ftpclient

import (
	"context"
	"fmt"
	"strings"
	"time"

	"auto-pack/backend/pkg/envtypes"
	"auto-pack/backend/pkg/probe"

	"github.com/jlaffaye/ftp"
)

// Verify connects to FTP and optionally lists base path.
func Verify(ctx context.Context, cfg envtypes.FTPConfig) probe.Result {
	cfg.Port = envtypes.DefaultPort(cfg.Port, 21)
	addr := fmt.Sprintf("%s:%d", strings.TrimSpace(cfg.Host), cfg.Port)
	details := map[string]string{
		"host": cfg.Host,
		"port": fmt.Sprintf("%d", cfg.Port),
	}

	timeout := time.Duration(envtypes.DefaultTimeoutSec(cfg.TimeoutSec)) * time.Second
	conn, err := ftp.Dial(addr, ftp.DialWithTimeout(timeout))
	if err != nil {
		return probe.Fail(fmt.Sprintf("ftp dial failed: %v", err), details)
	}
	defer conn.Quit()

	if err := conn.Login(strings.TrimSpace(cfg.Username), cfg.Password); err != nil {
		return probe.Fail(fmt.Sprintf("ftp login failed: %v", err), details)
	}

	base := strings.TrimSpace(cfg.BasePath)
	if base == "" {
		base = "."
	}
	details["base_path"] = base

	entries, err := conn.List(base)
	if err != nil {
		return probe.Fail(fmt.Sprintf("ftp list failed: %v", err), details)
	}
	details["entry_count"] = fmt.Sprintf("%d", len(entries))
	_ = ctx
	if cfg.Passive {
		details["passive"] = "true"
	}
	return probe.OK("FTP connection successful", details)
}
