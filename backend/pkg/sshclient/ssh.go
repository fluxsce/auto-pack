package sshclient

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	"auto-pack/backend/pkg/envtypes"
	"auto-pack/backend/pkg/probe"

	"golang.org/x/crypto/ssh"
)

// Verify dials SSH and opens a session to validate credentials.
func Verify(ctx context.Context, cfg envtypes.SSHConfig) probe.Result {
	cfg.Port = envtypes.DefaultPort(cfg.Port, 22)
	addr := fmt.Sprintf("%s:%d", strings.TrimSpace(cfg.Host), cfg.Port)
	details := map[string]string{
		"host": cfg.Host,
		"port": fmt.Sprintf("%d", cfg.Port),
	}

	dialer := &net.Dialer{Timeout: TimeoutDuration(cfg.TimeoutSec)}
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return probe.Fail(fmt.Sprintf("tcp dial failed: %v", err), details)
	}
	defer conn.Close()

	clientConfig, err := clientConfig(cfg)
	if err != nil {
		return probe.Fail(err.Error(), details)
	}

	sshConn, chans, reqs, err := ssh.NewClientConn(conn, addr, clientConfig)
	if err != nil {
		return probe.Fail(fmt.Sprintf("ssh handshake failed: %v", err), details)
	}
	defer sshConn.Close()

	client := ssh.NewClient(sshConn, chans, reqs)
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return probe.Fail(fmt.Sprintf("ssh session failed: %v", err), details)
	}
	defer session.Close()

	if err := session.Run("echo autopack-ssh-ok"); err != nil {
		return probe.Fail(fmt.Sprintf("ssh exec failed: %v", err), details)
	}
	return probe.OK("SSH connection successful", details)
}

// TimeoutDuration converts configured seconds to time.Duration.
func TimeoutDuration(sec int) time.Duration {
	if sec <= 0 {
		return 10 * time.Second
	}
	return time.Duration(sec) * time.Second
}
