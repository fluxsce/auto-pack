package sshclient

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	"auto-pack/backend/pkg/envtypes"

	"golang.org/x/crypto/ssh"
)

// CommandResult holds remote command output.
type CommandResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

// RunCommand executes command on the remote host via login shell.
func RunCommand(ctx context.Context, cfg envtypes.SSHConfig, command string) (CommandResult, error) {
	command = strings.TrimSpace(command)
	if command == "" {
		return CommandResult{}, fmt.Errorf("ssh: empty command")
	}

	client, err := Dial(ctx, cfg)
	if err != nil {
		return CommandResult{}, err
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return CommandResult{}, fmt.Errorf("ssh session: %w", err)
	}
	defer session.Close()

	var stdout, stderr bytes.Buffer
	session.Stdout = &stdout
	session.Stderr = &stderr

	remote := "bash -lc " + shellQuote(command)
	if err := session.Run(remote); err != nil {
		if exitErr, ok := err.(*ssh.ExitError); ok {
			return CommandResult{
				Stdout:   stdout.String(),
				Stderr:   stderr.String(),
				ExitCode: exitErr.ExitStatus(),
			}, nil
		}
		return CommandResult{
			Stdout: stdout.String(),
			Stderr: stderr.String(),
		}, err
	}
	return CommandResult{Stdout: stdout.String(), Stderr: stderr.String(), ExitCode: 0}, nil
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'"'"'`) + "'"
}
