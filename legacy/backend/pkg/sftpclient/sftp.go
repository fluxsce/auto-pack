package sftpclient

import (
	"context"
	"fmt"
	"net"
	"strings"

	"auto-pack/backend/pkg/envtypes"
	"auto-pack/backend/pkg/probe"
	"auto-pack/backend/pkg/sshclient"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

// Verify connects via SSH and lists the configured remote path.
func Verify(ctx context.Context, cfg envtypes.SFTPConfig) probe.Result {
	cfg.Port = envtypes.DefaultPort(cfg.Port, 22)
	addr := fmt.Sprintf("%s:%d", strings.TrimSpace(cfg.Host), cfg.Port)
	details := map[string]string{
		"host": cfg.Host,
		"port": fmt.Sprintf("%d", cfg.Port),
	}

	sshCfg := envtypes.SSHConfig{
		Host:           cfg.Host,
		Port:           cfg.Port,
		Username:       cfg.Username,
		Password:       cfg.Password,
		PrivateKeyPath: cfg.PrivateKeyPath,
		TimeoutSec:     cfg.TimeoutSec,
	}
	clientConfig, err := sshclient.ExportClientConfig(sshCfg)
	if err != nil {
		return probe.Fail(err.Error(), details)
	}

	dialer := &net.Dialer{Timeout: sshclient.TimeoutDuration(cfg.TimeoutSec)}
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return probe.Fail(fmt.Sprintf("tcp dial failed: %v", err), details)
	}
	defer conn.Close()

	sshConn, chans, reqs, err := ssh.NewClientConn(conn, addr, clientConfig)
	if err != nil {
		return probe.Fail(fmt.Sprintf("ssh handshake failed: %v", err), details)
	}
	defer sshConn.Close()

	client := ssh.NewClient(sshConn, chans, reqs)
	defer client.Close()

	sftpClient, err := sftp.NewClient(client)
	if err != nil {
		return probe.Fail(fmt.Sprintf("sftp client failed: %v", err), details)
	}
	defer sftpClient.Close()

	remote := strings.TrimSpace(cfg.RemotePath)
	if remote == "" {
		remote = "."
	}
	details["remote_path"] = remote

	entries, err := sftpClient.ReadDir(remote)
	if err != nil {
		return probe.Fail(fmt.Sprintf("sftp read dir failed: %v", err), details)
	}
	details["entry_count"] = fmt.Sprintf("%d", len(entries))
	return probe.OK("SFTP connection successful", details)
}
