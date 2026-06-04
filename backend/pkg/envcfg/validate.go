package envcfg

import (
	"fmt"
	"strings"

	"auto-pack/backend/pkg/email"
	"auto-pack/backend/pkg/envtypes"
)

func ValidateSSH(c SSHConfig) error {
	if strings.TrimSpace(c.Host) == "" {
		return fmt.Errorf("ssh: host is required")
	}
	if strings.TrimSpace(c.Username) == "" {
		return fmt.Errorf("ssh: username is required")
	}
	if strings.TrimSpace(c.Password) == "" && strings.TrimSpace(c.PrivateKeyPath) == "" {
		return fmt.Errorf("ssh: password or private_key_path is required")
	}
	return nil
}

func ValidateSFTP(c SFTPConfig) error {
	if strings.TrimSpace(c.Host) == "" {
		return fmt.Errorf("sftp: host is required")
	}
	if strings.TrimSpace(c.Username) == "" {
		return fmt.Errorf("sftp: username is required")
	}
	if strings.TrimSpace(c.Password) == "" && strings.TrimSpace(c.PrivateKeyPath) == "" {
		return fmt.Errorf("sftp: password or private_key_path is required")
	}
	return nil
}

func ValidateFTP(c FTPConfig) error {
	if strings.TrimSpace(c.Host) == "" {
		return fmt.Errorf("ftp: host is required")
	}
	if strings.TrimSpace(c.Username) == "" {
		return fmt.Errorf("ftp: username is required")
	}
	return nil
}

func ValidateGit(c GitConfig) error {
	auth := strings.ToLower(strings.TrimSpace(c.AuthType))
	if auth == "" || auth == envtypes.GitAuthNone {
		return nil
	}
	switch auth {
	case envtypes.GitAuthHTTPS:
		if strings.TrimSpace(c.AuthToken) != "" {
			return nil
		}
		if strings.TrimSpace(c.AuthUsername) == "" {
			return fmt.Errorf("git: auth_username is required for https (or set auth_token)")
		}
		if strings.TrimSpace(c.AuthPassword) == "" {
			return fmt.Errorf("git: auth_password or auth_token is required for https")
		}
	case envtypes.GitAuthSSH:
		if strings.TrimSpace(c.PrivateKeyPath) == "" {
			return fmt.Errorf("git: private_key_path is required for ssh auth")
		}
	default:
		return fmt.Errorf("git: invalid auth_type %q", c.AuthType)
	}
	return nil
}

func ValidateMaven(_ MavenConfig) error {
	return nil
}

func ValidateNodeJS(_ NodeJSConfig) error {
	return nil
}

func ValidateJDK(_ JDKConfig) error {
	return nil
}

func ValidateSMTP(c SMTPConfig) error {
	cfg := email.Config{
		Host:               c.Host,
		Port:               c.Port,
		Username:           c.Username,
		Password:           c.Password,
		From:               c.From,
		FromName:           c.FromName,
		TLS:                c.TLS,
		StartTLS:           c.StartTLS,
		InsecureSkipVerify: c.InsecureSkipVerify,
		TimeoutSec:         c.TimeoutSec,
	}
	return cfg.Validate()
}
