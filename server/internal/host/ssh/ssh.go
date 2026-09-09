// Package ssh 拨号并走 SFTP 上传。无 HTTP、无权限表。
package ssh

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	cryptossh "golang.org/x/crypto/ssh"
)

// Config is an SSH / SFTP endpoint.
type Config struct {
	Host       string
	Port       string
	Username   string
	Password   string
	KeyPath    string
	Passphrase string
	RemotePath string
	Timeout    time.Duration
}

// Dial opens an SSH client. Host keys are not verified (job sandbox).
func Dial(cfg Config) (*cryptossh.Client, error) {
	if strings.TrimSpace(cfg.Host) == "" {
		return nil, fmt.Errorf("ssh: host required")
	}
	port := strings.TrimSpace(cfg.Port)
	if port == "" {
		port = "22"
	}
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	auth, err := authMethods(cfg)
	if err != nil {
		return nil, err
	}
	if len(auth) == 0 {
		return nil, fmt.Errorf("ssh: password or private_key_path required")
	}
	user := strings.TrimSpace(cfg.Username)
	if user == "" {
		return nil, fmt.Errorf("ssh: username required")
	}
	return cryptossh.Dial("tcp", net.JoinHostPort(cfg.Host, port), &cryptossh.ClientConfig{
		User:            user,
		Auth:            auth,
		HostKeyCallback: cryptossh.InsecureIgnoreHostKey(),
		Timeout:         timeout,
	})
}

// MapConfig reads a decrypted ssh/sftp env config map.
func MapConfig(m map[string]any) Config {
	if m == nil {
		return Config{Timeout: 60 * time.Second}
	}
	sec := asInt(m["timeout_sec"])
	timeout := time.Duration(sec) * time.Second
	if sec <= 0 {
		timeout = 60 * time.Second
	}
	return Config{
		Host:       asString(m["host"]),
		Port:       asString(m["port"]),
		Username:   asString(m["username"]),
		Password:   asString(m["password"]),
		KeyPath:    asString(m["private_key_path"]),
		Passphrase: asString(m["private_key_passphrase"]),
		RemotePath: asString(m["remote_path"], m["remote_dir"], m["base_path"]),
		Timeout:    timeout,
	}
}

func authMethods(cfg Config) ([]cryptossh.AuthMethod, error) {
	var auth []cryptossh.AuthMethod
	if cfg.Password != "" {
		auth = append(auth, cryptossh.Password(cfg.Password))
	}
	if cfg.KeyPath == "" {
		return auth, nil
	}
	raw, err := os.ReadFile(cfg.KeyPath)
	if err != nil {
		return nil, err
	}
	var signer cryptossh.Signer
	if cfg.Passphrase != "" {
		signer, err = cryptossh.ParsePrivateKeyWithPassphrase(raw, []byte(cfg.Passphrase))
	} else {
		signer, err = cryptossh.ParsePrivateKey(raw)
	}
	if err != nil {
		return nil, err
	}
	return append(auth, cryptossh.PublicKeys(signer)), nil
}

func asString(values ...any) string {
	for _, v := range values {
		if s := anyString(v); s != "" {
			return s
		}
	}
	return ""
}

func asInt(v any) int {
	switch t := v.(type) {
	case float64:
		return int(t)
	case int:
		return t
	case int64:
		return int(t)
	case string:
		n, _ := strconv.Atoi(strings.TrimSpace(t))
		return n
	default:
		return 0
	}
}

func anyString(v any) string {
	switch t := v.(type) {
	case string:
		return strings.TrimSpace(t)
	case float64:
		if t == float64(int64(t)) {
			return strconv.FormatInt(int64(t), 10)
		}
		return strconv.FormatFloat(t, 'f', -1, 64)
	case int:
		return strconv.Itoa(t)
	case int64:
		return strconv.FormatInt(t, 10)
	default:
		return ""
	}
}
