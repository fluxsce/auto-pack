package env

import (
	"context"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"auto-pack/server/internal/host/ftp"
	hostsmtp "auto-pack/server/internal/host/smtp"
	"auto-pack/server/internal/iam"

	ftpclient "github.com/jlaffaye/ftp"
	"golang.org/x/crypto/ssh"
)

const probeTimeout = 8 * time.Second
const smtpProbeTimeout = 20 * time.Second

// ProbeResult is a connectivity check. Failed tests still return HTTP 200.
type ProbeResult struct {
	OK      bool              `json:"ok"`
	Message string            `json:"message"`
	Details map[string]string `json:"details,omitempty"`
}

// ProbeIn is a draft or saved-row test. rec_id 有值时会合并已存密钥。
// TestTo 只用于 SMTP 探测发信，不写入环境配置。
type ProbeIn struct {
	WorkspaceRecID string         `json:"workspace_rec_id"`
	RecID          string         `json:"rec_id"`
	EnvKind        string         `json:"env_kind"`
	Config         map[string]any `json:"config"`
	TestTo         string         `json:"test_to"`
}

// Probe checks an env draft or saved row (env.write). It does not persist.
func (s *Service) Probe(ctx context.Context, actor iam.Actor, in ProbeIn) (ProbeResult, error) {
	if err := s.authorizeWrite(ctx, actor, in.WorkspaceRecID); err != nil {
		return ProbeResult{}, err
	}
	kind, cfg, err := s.prepareProbeConfig(ctx, in)
	if err != nil {
		return ProbeResult{}, err
	}
	timeout := probeTimeout
	if kind == "smtp" {
		timeout = smtpProbeTimeout
	}
	pctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return runProbe(pctx, kind, cfg, in.TestTo), nil
}

func (s *Service) prepareProbeConfig(ctx context.Context, in ProbeIn) (string, map[string]any, error) {
	kind := strings.TrimSpace(in.EnvKind)
	cfg := in.Config
	if in.RecID != "" {
		existing, cipherText, err := s.loadRow(ctx, in.RecID)
		if err != nil {
			return "", nil, err
		}
		if kind == "" {
			kind = existing.EnvKind
		}
		old, err := s.decryptMap(cipherText)
		if err != nil {
			return "", nil, err
		}
		norm, err := NormalizeKind(kind)
		if err != nil {
			return "", nil, err
		}
		if !hasProbeOverlay(cfg) {
			return norm, old, nil
		}
		incoming, err := NormalizeConfig(norm, cfg)
		if err != nil {
			return "", nil, err
		}
		return norm, overlayProbeConfig(norm, incoming, old), nil
	}
	norm, err := NormalizeKind(kind)
	if err != nil {
		return "", nil, err
	}
	out, err := NormalizeConfig(norm, cfg)
	if err != nil {
		return "", nil, err
	}
	return norm, out, nil
}

func runProbe(ctx context.Context, kind string, cfg map[string]any, testTo string) ProbeResult {
	switch kind {
	case "ssh", "sftp":
		return probeSSH(ctx, cfg)
	case "ftp":
		return probeFTP(ctx, cfg)
	case "smtp":
		return probeSMTP(ctx, cfg, testTo)
	case "git":
		return probeBinary(ctx, firstNonEmpty(asString(cfg["git_path"]), "git"), "--version")
	case "jdk":
		return probeHomeOrBin(ctx, asString(cfg["java_home"]), asString(cfg["java_path"]), "java", "-version")
	case "maven":
		return probeHomeOrBin(ctx, asString(cfg["maven_home"]), asString(cfg["mvn_path"]), "mvn", "-v")
	case "node":
		return probeHomeOrBin(ctx, "", asString(cfg["node_path"]), "node", "-v")
	default:
		return ProbeResult{OK: false, Message: "probe not supported"}
	}
}

func probeSSH(ctx context.Context, cfg map[string]any) ProbeResult {
	host := asString(cfg["host"])
	if host == "" {
		return ProbeResult{OK: false, Message: "host required"}
	}
	port := firstNonEmpty(asString(cfg["port"]), "22")
	addr := net.JoinHostPort(host, port)
	d := net.Dialer{}
	conn, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return ProbeResult{OK: false, Message: err.Error()}
	}
	user := asString(cfg["username"])
	if user == "" {
		_ = conn.Close()
		return ProbeResult{OK: true, Message: "tcp ok", Details: map[string]string{"addr": addr}}
	}
	auth, err := sshAuth(cfg)
	if err != nil {
		_ = conn.Close()
		return ProbeResult{OK: false, Message: err.Error()}
	}
	if len(auth) == 0 {
		_ = conn.Close()
		return ProbeResult{OK: true, Message: "tcp ok, no auth", Details: map[string]string{"addr": addr}}
	}
	deadline := time.Now().Add(probeTimeout)
	if t, ok := ctx.Deadline(); ok {
		deadline = t
	}
	_ = conn.SetDeadline(deadline)
	c, chans, reqs, err := ssh.NewClientConn(conn, addr, &ssh.ClientConfig{
		User:            user,
		Auth:            auth,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         probeTimeout,
	})
	if err != nil {
		return ProbeResult{OK: false, Message: err.Error()}
	}
	client := ssh.NewClient(c, chans, reqs)
	_ = client.Close()
	return ProbeResult{OK: true, Message: "ssh ok", Details: map[string]string{"addr": addr, "user": user}}
}

func sshAuth(cfg map[string]any) ([]ssh.AuthMethod, error) {
	var auth []ssh.AuthMethod
	if pw := asString(cfg["password"]); pw != "" {
		auth = append(auth, ssh.Password(pw))
	}
	keyPath := asString(cfg["private_key_path"])
	if keyPath == "" {
		return auth, nil
	}
	raw, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, err
	}
	var signer ssh.Signer
	if pass := asString(cfg["private_key_passphrase"]); pass != "" {
		signer, err = ssh.ParsePrivateKeyWithPassphrase(raw, []byte(pass))
	} else {
		signer, err = ssh.ParsePrivateKey(raw)
	}
	if err != nil {
		return nil, err
	}
	return append(auth, ssh.PublicKeys(signer)), nil
}

func probeFTP(ctx context.Context, cfg map[string]any) ProbeResult {
	host := asString(cfg["host"])
	if host == "" {
		return ProbeResult{OK: false, Message: "host required"}
	}
	fc := ftp.MapConfig(cfg)
	addr := fc.Host
	if fc.Port != "" {
		addr = net.JoinHostPort(fc.Host, fc.Port)
	} else if !strings.Contains(addr, ":") {
		addr = net.JoinHostPort(fc.Host, "21")
	}
	c, err := ftpclient.Dial(addr, ftpclient.DialWithContext(ctx))
	if err != nil {
		return ProbeResult{OK: false, Message: err.Error()}
	}
	defer func() { _ = c.Quit() }()
	if fc.Username != "" {
		if err := c.Login(fc.Username, fc.Password); err != nil {
			return ProbeResult{OK: false, Message: err.Error()}
		}
	}
	return ProbeResult{OK: true, Message: "ftp ok", Details: map[string]string{"addr": addr}}
}

func probeSMTP(ctx context.Context, cfg map[string]any, testTo string) ProbeResult {
	sc := hostsmtp.MapConfig(cfg)
	if strings.TrimSpace(sc.Host) == "" {
		return ProbeResult{OK: false, Message: "host required"}
	}
	to := strings.TrimSpace(testTo)
	if to == "" {
		return ProbeResult{OK: false, Message: "test recipient required"}
	}
	sc.To = to
	if deadline, ok := ctx.Deadline(); ok {
		remain := time.Until(deadline)
		if remain <= 0 {
			return ProbeResult{OK: false, Message: context.DeadlineExceeded.Error()}
		}
		if sc.Timeout <= 0 || remain < sc.Timeout {
			sc.Timeout = remain
		}
	}
	if err := hostsmtp.Send(sc, "Auto Pack SMTP test", "SMTP connection test."); err != nil {
		return ProbeResult{OK: false, Message: err.Error()}
	}
	return ProbeResult{OK: true, Message: "smtp test mail sent", Details: map[string]string{"addr": sc.Addr(), "to": to}}
}

func probeHomeOrBin(ctx context.Context, home, bin, fallback string, args ...string) ProbeResult {
	if home != "" {
		st, err := os.Stat(home)
		if err != nil {
			return ProbeResult{OK: false, Message: err.Error()}
		}
		if !st.IsDir() {
			return ProbeResult{OK: false, Message: "home is not a directory"}
		}
		if bin == "" {
			cand := filepath.Join(home, "bin", fallback)
			if _, err := os.Stat(cand); err == nil {
				return probeBinary(ctx, cand, args...)
			}
			return ProbeResult{OK: true, Message: "home ok", Details: map[string]string{"home": home}}
		}
	}
	return probeBinary(ctx, firstNonEmpty(bin, fallback), args...)
}

func probeBinary(ctx context.Context, name string, args ...string) ProbeResult {
	name = strings.TrimSpace(name)
	if name == "" {
		return ProbeResult{OK: false, Message: "binary path required"}
	}
	path := name
	if !strings.ContainsAny(name, `/\`) {
		looked, err := exec.LookPath(name)
		if err != nil {
			return ProbeResult{OK: false, Message: err.Error()}
		}
		path = looked
	} else if _, err := os.Stat(name); err != nil {
		return ProbeResult{OK: false, Message: err.Error()}
	}
	cmd := exec.CommandContext(ctx, path, args...)
	out, err := cmd.CombinedOutput()
	msg := strings.TrimSpace(string(out))
	if err != nil && msg == "" {
		return ProbeResult{OK: false, Message: err.Error()}
	}
	if err != nil {
		return ProbeResult{OK: false, Message: msg}
	}
	if msg == "" {
		msg = path
	}
	return ProbeResult{OK: true, Message: firstLine(msg), Details: map[string]string{"path": path}}
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexAny(s, "\r\n"); i >= 0 {
		return s[:i]
	}
	return s
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func hasProbeOverlay(cfg map[string]any) bool {
	for _, v := range cfg {
		if !isEmpty(v) {
			return true
		}
	}
	return false
}

// overlayProbeConfig starts from stored config so list-row tests keep host/path.
// Incoming non-empty fields win; empty secrets stay on the stored values.
func overlayProbeConfig(kind string, incoming, existing map[string]any) map[string]any {
	out := map[string]any{}
	for k, v := range existing {
		out[k] = v
	}
	for k, v := range incoming {
		if isEmpty(v) {
			continue
		}
		out[k] = v
	}
	return MergeSecrets(kind, out, existing)
}
