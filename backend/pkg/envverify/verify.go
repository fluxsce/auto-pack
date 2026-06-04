package envverify

import (
	"context"

	"auto-pack/backend/pkg/email"
	"auto-pack/backend/pkg/envcfg"
	"auto-pack/backend/pkg/ftpclient"
	"auto-pack/backend/pkg/git"
	"auto-pack/backend/pkg/jdk"
	"auto-pack/backend/pkg/maven"
	"auto-pack/backend/pkg/nodejs"
	"auto-pack/backend/pkg/probe"
	"auto-pack/backend/pkg/sftpclient"
	"auto-pack/backend/pkg/sshclient"
)

// Run executes a connectivity or binary probe for envType and config JSON.
func Run(ctx context.Context, envType, configJSON string) (probe.Result, error) {
	parsed, err := envcfg.Parse(envType, configJSON)
	if err != nil {
		return probe.Fail(err.Error(), nil), err
	}
	switch envType {
	case envcfg.TypeSSH:
		return sshclient.Verify(ctx, parsed.(envcfg.SSHConfig)), nil
	case envcfg.TypeSFTP:
		return sftpclient.Verify(ctx, parsed.(envcfg.SFTPConfig)), nil
	case envcfg.TypeFTP:
		return ftpclient.Verify(ctx, parsed.(envcfg.FTPConfig)), nil
	case envcfg.TypeGit:
		return git.VerifyEnv(ctx, parsed.(envcfg.GitConfig)), nil
	case envcfg.TypeMaven:
		return maven.Verify(ctx, parsed.(envcfg.MavenConfig)), nil
	case envcfg.TypeNodeJS:
		return nodejs.Verify(ctx, parsed.(envcfg.NodeJSConfig)), nil
	case envcfg.TypeJDK:
		return jdk.Verify(ctx, parsed.(envcfg.JDKConfig)), nil
	case envcfg.TypeSMTP:
		cfg := parsed.(envcfg.SMTPConfig)
		emailCfg := email.Config{
			Host:               cfg.Host,
			Port:               cfg.Port,
			Username:           cfg.Username,
			Password:           cfg.Password,
			From:               cfg.From,
			FromName:           cfg.FromName,
			TLS:                cfg.TLS,
			StartTLS:           cfg.StartTLS,
			InsecureSkipVerify: cfg.InsecureSkipVerify,
			TimeoutSec:         cfg.TimeoutSec,
		}
		return email.Verify(ctx, emailCfg), nil
	default:
		return probe.Fail("unknown env_type", nil), nil
	}
}
