package envcfg

import "auto-pack/backend/pkg/envtypes"

// Re-export env type constants and config structs for callers using envcfg.
const (
	TypeSSH    = envtypes.TypeSSH
	TypeSFTP   = envtypes.TypeSFTP
	TypeFTP    = envtypes.TypeFTP
	TypeGit    = envtypes.TypeGit
	TypeMaven  = envtypes.TypeMaven
	TypeNodeJS = envtypes.TypeNodeJS
	TypeJDK    = envtypes.TypeJDK
	TypeSMTP   = envtypes.TypeSMTP
)

var AllTypes = envtypes.AllTypes

type (
	SSHConfig    = envtypes.SSHConfig
	SFTPConfig   = envtypes.SFTPConfig
	FTPConfig    = envtypes.FTPConfig
	GitConfig    = envtypes.GitConfig
	MavenConfig  = envtypes.MavenConfig
	NodeJSConfig = envtypes.NodeJSConfig
	JDKConfig    = envtypes.JDKConfig
	SMTPConfig   = envtypes.SMTPConfig
)

// DefaultPort returns port or fallback when port <= 0.
func DefaultPort(port, fallback int) int { return envtypes.DefaultPort(port, fallback) }

// DefaultTimeoutSec returns sec or 60 when sec <= 0.
func DefaultTimeoutSec(sec int) int { return envtypes.DefaultTimeoutSec(sec) }
