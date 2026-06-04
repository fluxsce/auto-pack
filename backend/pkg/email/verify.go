package email

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/smtp"
	"strings"
	"time"

	"auto-pack/backend/pkg/probe"
)

// Verify checks SMTP connectivity and optional authentication.
func Verify(ctx context.Context, cfg Config) probe.Result {
	cfg.Normalize()
	details := map[string]string{
		"host": cfg.Host,
		"port": fmt.Sprintf("%d", cfg.Port),
	}
	if cfg.From != "" {
		details["from"] = cfg.From
	}
	if cfg.TLS {
		details["tls"] = "true"
	}
	if cfg.StartTLS {
		details["starttls"] = "true"
	}

	if err := cfg.Validate(); err != nil {
		return probe.Fail(err.Error(), details)
	}

	client, err := openSMTP(ctx, cfg)
	if err != nil {
		return probe.Fail(err.Error(), details)
	}
	defer client.Close()

	if err := client.Noop(); err != nil {
		return probe.Fail(fmt.Sprintf("smtp noop failed: %v", err), details)
	}
	_ = client.Quit()
	return probe.OK("SMTP connection successful", details)
}

func openSMTP(ctx context.Context, cfg Config) (*smtp.Client, error) {
	timeout := time.Duration(cfg.Timeout()) * time.Second
	host := cfgHost(cfg)
	addr := cfg.Addr()

	if cfg.TLS {
		dialer := &net.Dialer{Timeout: timeout}
		conn, err := dialer.DialContext(ctx, "tcp", addr)
		if err != nil {
			return nil, fmt.Errorf("dial %s: %w", addr, err)
		}
		tlsCfg := &tls.Config{
			ServerName:         host,
			InsecureSkipVerify: cfg.InsecureSkipVerify,
			MinVersion:         tls.VersionTLS12,
		}
		tlsConn := tls.Client(conn, tlsCfg)
		if err := tlsConn.HandshakeContext(ctx); err != nil {
			_ = conn.Close()
			return nil, fmt.Errorf("tls handshake: %w", err)
		}
		client, err := smtp.NewClient(tlsConn, host)
		if err != nil {
			return nil, fmt.Errorf("smtp client: %w", err)
		}
		if err := verifyAuth(cfg, client); err != nil {
			_ = client.Close()
			return nil, err
		}
		return client, nil
	}

	dialer := &net.Dialer{Timeout: timeout}
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("dial %s: %w", addr, err)
	}
	client, err := smtp.NewClient(conn, host)
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("smtp client: %w", err)
	}

	if cfg.StartTLS {
		if ok, _ := client.Extension("STARTTLS"); ok {
			tlsCfg := &tls.Config{
				ServerName:         host,
				InsecureSkipVerify: cfg.InsecureSkipVerify,
				MinVersion:         tls.VersionTLS12,
			}
			if err := client.StartTLS(tlsCfg); err != nil {
				_ = client.Close()
				return nil, fmt.Errorf("starttls: %w", err)
			}
		}
	}

	if err := verifyAuth(cfg, client); err != nil {
		_ = client.Close()
		return nil, err
	}
	return client, nil
}

func verifyAuth(cfg Config, client *smtp.Client) error {
	user := strings.TrimSpace(cfg.Username)
	if user == "" {
		return nil
	}
	if cfg.Password == "" {
		return fmt.Errorf("smtp password is required when username is set")
	}
	c := &Client{cfg: cfg}
	return c.authenticate(client)
}
