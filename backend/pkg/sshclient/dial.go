package sshclient

import (
	"context"
	"fmt"
	"net"
	"strings"

	"auto-pack/backend/pkg/envtypes"

	"golang.org/x/crypto/ssh"
)

// Dial opens an SSH client connection.
func Dial(ctx context.Context, cfg envtypes.SSHConfig) (*ssh.Client, error) {
	cfg.Port = envtypes.DefaultPort(cfg.Port, 22)
	addr := fmt.Sprintf("%s:%d", strings.TrimSpace(cfg.Host), cfg.Port)

	clientConfig, err := clientConfig(cfg)
	if err != nil {
		return nil, err
	}

	dialer := &net.Dialer{Timeout: TimeoutDuration(cfg.TimeoutSec)}
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("tcp dial %s: %w", addr, err)
	}

	sshConn, chans, reqs, err := ssh.NewClientConn(conn, addr, clientConfig)
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("ssh handshake: %w", err)
	}
	return ssh.NewClient(sshConn, chans, reqs), nil
}
