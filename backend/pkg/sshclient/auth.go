package sshclient

import (
	"fmt"
	"os"
	"strings"

	"auto-pack/backend/pkg/envtypes"

	"golang.org/x/crypto/ssh"
)

func buildAuthMethods(cfg envtypes.SSHConfig) ([]ssh.AuthMethod, error) {
	var methods []ssh.AuthMethod
	if pw := strings.TrimSpace(cfg.Password); pw != "" {
		methods = append(methods, ssh.Password(pw))
	}
	if keyPath := strings.TrimSpace(cfg.PrivateKeyPath); keyPath != "" {
		key, err := os.ReadFile(keyPath)
		if err != nil {
			return nil, fmt.Errorf("read private key: %w", err)
		}
		signer, err := ssh.ParsePrivateKey(key)
		if err != nil {
			return nil, fmt.Errorf("parse private key: %w", err)
		}
		methods = append(methods, ssh.PublicKeys(signer))
	}
	if len(methods) == 0 {
		return nil, fmt.Errorf("no ssh credentials configured")
	}
	return methods, nil
}

// ExportClientConfig builds ssh.ClientConfig for reuse by SFTP.
func ExportClientConfig(cfg envtypes.SSHConfig) (*ssh.ClientConfig, error) {
	return clientConfig(cfg)
}

func clientConfig(cfg envtypes.SSHConfig) (*ssh.ClientConfig, error) {
	methods, err := buildAuthMethods(cfg)
	if err != nil {
		return nil, err
	}
	return &ssh.ClientConfig{
		User:            strings.TrimSpace(cfg.Username),
		Auth:            methods,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), //nolint:gosec // admin-configured env probe
		Timeout:         TimeoutDuration(cfg.TimeoutSec),
	}, nil
}
