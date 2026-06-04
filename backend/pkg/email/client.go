package email

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/smtp"
	"strings"
	"time"
)

// Client sends email over SMTP.
type Client struct {
	cfg Config
}

// New validates config and returns a Client.
func New(cfg Config) (*Client, error) {
	cfg.Normalize()
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return &Client{cfg: cfg}, nil
}

// Config returns a copy of the client configuration.
func (c *Client) Config() Config {
	return c.cfg
}

// Send delivers msg using SMTP. Context cancels dial and handshake waits.
func (c *Client) Send(ctx context.Context, msg Message) error {
	if err := msg.Validate(); err != nil {
		return err
	}
	raw, err := buildMIME(c.cfg, msg)
	if err != nil {
		return err
	}
	return c.deliver(ctx, msg.AllRecipients(), raw)
}

func (c *Client) deliver(ctx context.Context, recipients []string, raw []byte) error {
	cfg := c.cfg
	cfg.Normalize()

	if cfg.TLS {
		return c.sendTLS(ctx, recipients, raw)
	}
	return c.sendPlain(ctx, recipients, raw)
}

func (c *Client) sendPlain(ctx context.Context, recipients []string, raw []byte) error {
	conn, err := c.dial(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()

	host := cfgHost(c.cfg)
	client, err := smtp.NewClient(conn, host)
	if err != nil {
		return fmt.Errorf("smtp client: %w", err)
	}
	defer client.Close()

	if c.cfg.StartTLS {
		if ok, _ := client.Extension("STARTTLS"); ok {
			tlsCfg := c.tlsConfig(host)
			if err := client.StartTLS(tlsCfg); err != nil {
				return fmt.Errorf("starttls: %w", err)
			}
		}
	}

	if err := c.authenticate(client); err != nil {
		return err
	}
	return c.submit(client, recipients, raw)
}

func (c *Client) sendTLS(ctx context.Context, recipients []string, raw []byte) error {
	host := cfgHost(c.cfg)
	tlsCfg := c.tlsConfig(host)
	dialer := &net.Dialer{Timeout: time.Duration(c.cfg.Timeout()) * time.Second}
	conn, err := dialer.DialContext(ctx, "tcp", c.cfg.Addr())
	if err != nil {
		return fmt.Errorf("dial %s: %w", c.cfg.Addr(), err)
	}
	defer conn.Close()

	tlsConn := tls.Client(conn, tlsCfg)
	if err := tlsConn.HandshakeContext(ctx); err != nil {
		return fmt.Errorf("tls handshake: %w", err)
	}

	client, err := smtp.NewClient(tlsConn, host)
	if err != nil {
		return fmt.Errorf("smtp client: %w", err)
	}
	defer client.Close()

	if err := c.authenticate(client); err != nil {
		return err
	}
	return c.submit(client, recipients, raw)
}

func (c *Client) dial(ctx context.Context) (net.Conn, error) {
	dialer := &net.Dialer{Timeout: time.Duration(c.cfg.Timeout()) * time.Second}
	conn, err := dialer.DialContext(ctx, "tcp", c.cfg.Addr())
	if err != nil {
		return nil, fmt.Errorf("dial %s: %w", c.cfg.Addr(), err)
	}
	return conn, nil
}

func (c *Client) tlsConfig(serverName string) *tls.Config {
	return &tls.Config{
		ServerName:         serverName,
		InsecureSkipVerify: c.cfg.InsecureSkipVerify,
		MinVersion:         tls.VersionTLS12,
	}
}

func (c *Client) authenticate(client *smtp.Client) error {
	user := strings.TrimSpace(c.cfg.Username)
	if user == "" {
		return nil
	}
	pass := c.cfg.Password
	if pass == "" {
		return fmt.Errorf("smtp password is required when username is set")
	}
	auth, err := c.smtpAuth(client, user, pass)
	if err != nil {
		return err
	}
	if auth == nil {
		return fmt.Errorf("smtp server does not support AUTH")
	}
	if err := client.Auth(auth); err != nil {
		return fmt.Errorf("smtp auth: %w", err)
	}
	return nil
}

func (c *Client) smtpAuth(client *smtp.Client, user, pass string) (smtp.Auth, error) {
	ok, mechList := client.Extension("AUTH")
	if !ok {
		return nil, nil
	}
	mechList = strings.ToUpper(mechList)
	switch {
	case strings.Contains(mechList, "PLAIN"):
		return smtp.PlainAuth("", user, pass, cfgHost(c.cfg)), nil
	case strings.Contains(mechList, "LOGIN"):
		return newLoginAuth(user, pass), nil
	default:
		return nil, fmt.Errorf("unsupported AUTH mechanisms: %s", mechList)
	}
}

func (c *Client) submit(client *smtp.Client, recipients []string, raw []byte) error {
	from := c.cfg.From
	if err := client.Mail(from); err != nil {
		return fmt.Errorf("mail from: %w", err)
	}
	for _, rcpt := range recipients {
		if err := client.Rcpt(rcpt); err != nil {
			return fmt.Errorf("rcpt %s: %w", rcpt, err)
		}
	}
	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("data: %w", err)
	}
	if _, err := w.Write(raw); err != nil {
		_ = w.Close()
		return fmt.Errorf("write message: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("close data: %w", err)
	}
	return client.Quit()
}

func cfgHost(cfg Config) string {
	return strings.TrimSpace(cfg.Host)
}

// loginAuth implements SMTP LOGIN authentication.
type loginAuth struct {
	username string
	password string
}

func newLoginAuth(username, password string) smtp.Auth {
	return loginAuth{username: username, password: password}
}

func (a loginAuth) Start(server *smtp.ServerInfo) (string, []byte, error) {
	if !server.TLS {
		return "", nil, fmt.Errorf("login auth requires TLS")
	}
	return "LOGIN", nil, nil
}

func (a loginAuth) Next(fromServer []byte, more bool) ([]byte, error) {
	if !more {
		return nil, nil
	}
	prompt := strings.ToLower(string(fromServer))
	switch {
	case strings.Contains(prompt, "username"):
		return []byte(a.username), nil
	case strings.Contains(prompt, "password"):
		return []byte(a.password), nil
	default:
		return nil, fmt.Errorf("unexpected login challenge: %s", fromServer)
	}
}
