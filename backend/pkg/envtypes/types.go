package envtypes

// Supported environment types stored in AP_SYS_ENV.env_type.
const (
	TypeSSH    = "ssh"
	TypeSFTP   = "sftp"
	TypeFTP    = "ftp"
	TypeGit    = "git"
	TypeMaven  = "maven"
	TypeNodeJS = "nodejs"
	TypeJDK    = "jdk"
	TypeSMTP   = "smtp"
)

// AllTypes lists valid env_type values.
var AllTypes = []string{TypeSSH, TypeSFTP, TypeFTP, TypeGit, TypeMaven, TypeNodeJS, TypeJDK, TypeSMTP}

// SMTPConfig is used for smtp env_type (post-build email notifications).
type SMTPConfig struct {
	Host               string `json:"host"`
	Port               int    `json:"port"`
	Username           string `json:"username,omitempty"`
	Password           string `json:"password,omitempty"`
	From               string `json:"from"`
	FromName           string `json:"from_name,omitempty"`
	TLS                bool   `json:"tls,omitempty"`
	StartTLS           bool   `json:"starttls,omitempty"`
	InsecureSkipVerify bool   `json:"insecure_skip_verify,omitempty"`
	TimeoutSec         int    `json:"timeout_sec,omitempty"`
}

// SSHConfig is used for ssh env_type.
type SSHConfig struct {
	Host           string `json:"host"`
	Port           int    `json:"port"`
	Username       string `json:"username"`
	Password       string `json:"password,omitempty"`
	PrivateKeyPath string `json:"private_key_path,omitempty"`
	TimeoutSec     int    `json:"timeout_sec,omitempty"`
}

// SFTPConfig is used for sftp env_type.
type SFTPConfig struct {
	Host           string `json:"host"`
	Port           int    `json:"port"`
	Username       string `json:"username"`
	Password       string `json:"password,omitempty"`
	PrivateKeyPath string `json:"private_key_path,omitempty"`
	RemotePath     string `json:"remote_path,omitempty"`
	TimeoutSec     int    `json:"timeout_sec,omitempty"`
}

// FTPConfig is used for ftp env_type.
type FTPConfig struct {
	Host       string `json:"host"`
	Port       int    `json:"port"`
	Username   string `json:"username"`
	Password   string `json:"password"`
	BasePath   string `json:"base_path,omitempty"`
	Passive    bool   `json:"passive,omitempty"`
	TimeoutSec int    `json:"timeout_sec,omitempty"`
}

// Git pull/fetch authentication modes (git env_type).
const (
	GitAuthNone  = "none"
	GitAuthHTTPS = "https"
	GitAuthSSH   = "ssh"
)

// GitConfig is used for git env_type (clone/pull credentials + optional local git identity).
type GitConfig struct {
	GitPath  string `json:"git_path,omitempty"`
	Username string `json:"username,omitempty"` // commit user.name
	Email    string `json:"email,omitempty"`    // commit user.email

	AuthType string `json:"auth_type,omitempty"` // none | https | ssh

	AuthUsername string `json:"auth_username,omitempty"`
	AuthPassword string `json:"auth_password,omitempty"`
	AuthToken    string `json:"auth_token,omitempty"`

	PrivateKeyPath       string `json:"private_key_path,omitempty"`
	PrivateKeyPassphrase string `json:"private_key_passphrase,omitempty"`
}

// MavenConfig is used for maven env_type.
type MavenConfig struct {
	MavenHome   string `json:"maven_home,omitempty"`
	MvnPath     string `json:"mvn_path,omitempty"`
	SettingsXML string `json:"settings_xml,omitempty"`
	JavaHome    string `json:"java_home,omitempty"`
}

// NodeJSConfig is used for nodejs env_type.
type NodeJSConfig struct {
	NodePath           string `json:"node_path,omitempty"`
	NpmPath            string `json:"npm_path,omitempty"`
	NpxPath            string `json:"npx_path,omitempty"`
	NpmrcPath          string `json:"npmrc_path,omitempty"`
	Registry           string `json:"registry,omitempty"`
	AuthToken          string `json:"auth_token,omitempty"`
	AuthUsername       string `json:"auth_username,omitempty"`
	AuthPassword       string `json:"auth_password,omitempty"`
	PnpmAllowAllBuilds *bool  `json:"pnpm_allow_all_builds,omitempty"`
}

// JDKConfig is used for jdk env_type.
type JDKConfig struct {
	JavaHome string `json:"java_home,omitempty"`
	JavaPath string `json:"java_path,omitempty"`
}
