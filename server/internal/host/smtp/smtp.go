// Package smtp 发作业通知邮件。无 HTTP、无权限表。
package smtp

import (
	"context"
	"crypto/tls"
	"fmt"
	"mime"
	"net"
	"net/smtp"
	"strconv"
	"strings"
	"time"
)

// Config is an SMTP endpoint. Empty To skips sending (engine treats as no-op).
type Config struct {
	Host     string
	Port     string
	From     string
	FromName string
	To       string
	Username string
	Password string
	TLS      bool
	StartTLS bool
	Timeout  time.Duration
}

// Recipients splits To on comma or semicolon.
func (c Config) Recipients() []string {
	return SplitTo(c.To)
}

// Addr is host:port after applying the same 465/587 defaults as Send.
func (c Config) Addr() string {
	_, _, addr, _, err := resolveEndpoint(c)
	if err != nil {
		return ""
	}
	return addr
}

// FromAddr returns the envelope from address.
func (c Config) FromAddr() string {
	if s := strings.TrimSpace(c.From); s != "" {
		return s
	}
	return strings.TrimSpace(c.Username)
}

// Check dials with the same TLS/STARTTLS rules as Send and authenticates
// when username is set. It does not send mail and does not require To.
func Check(ctx context.Context, cfg Config) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	host, port, addr, timeout, err := resolveEndpoint(cfg)
	if err != nil {
		return err
	}
	if deadline, ok := ctx.Deadline(); ok {
		remain := time.Until(deadline)
		if remain <= 0 {
			return context.DeadlineExceeded
		}
		if remain < timeout {
			timeout = remain
		}
	}
	c, closer, err := openClient(host, addr, implicitTLS(cfg.TLS, port), cfg.StartTLS, timeout)
	if err != nil {
		return err
	}
	defer closer()
	if auth := authOf(cfg, host); auth != nil {
		if err := c.Auth(auth); err != nil {
			return err
		}
	}
	return c.Quit()
}

// Send delivers a plain-text message. Returns an error when To is empty.
func Send(cfg Config, subject, body string) error {
	return SendMIME(cfg, subject, body, "")
}

// SendMIME delivers text and optional HTML (multipart/alternative when html is set).
func SendMIME(cfg Config, subject, text, html string) error {
	to := cfg.Recipients()
	if len(to) == 0 {
		return fmt.Errorf("smtp: no recipients")
	}
	from := cfg.FromAddr()
	if from == "" {
		return fmt.Errorf("smtp: from required")
	}
	host, port, addr, timeout, err := resolveEndpoint(cfg)
	if err != nil {
		return err
	}
	msg := buildMessage(cfg, from, to, subject, text, html)
	c, closer, err := openClient(host, addr, implicitTLS(cfg.TLS, port), cfg.StartTLS, timeout)
	if err != nil {
		return err
	}
	defer closer()
	return finish(c, authOf(cfg, host), from, to, msg)
}

// MapConfig reads a decrypted smtp env config map.
func MapConfig(m map[string]any) Config {
	if m == nil {
		return Config{StartTLS: true, Timeout: 60 * time.Second}
	}
	sec := asInt(m["timeout_sec"])
	timeout := time.Duration(sec) * time.Second
	if sec <= 0 {
		timeout = 60 * time.Second
	}
	return Config{
		Host:     asString(m["host"]),
		Port:     asString(m["port"]),
		From:     asString(m["from"]),
		FromName: asString(m["from_name"]),
		To:       asString(m["to"]),
		Username: asString(m["username"]),
		Password: asString(m["password"]),
		TLS:      asBool(m["tls"]),
		StartTLS: asBoolDefault(m["starttls"], true),
		Timeout:  timeout,
	}
}

// SplitTo parses a recipient list.
func SplitTo(raw string) []string {
	raw = strings.ReplaceAll(raw, ";", ",")
	var out []string
	seen := map[string]bool{}
	for _, part := range strings.Split(raw, ",") {
		addr := strings.TrimSpace(part)
		if addr == "" || seen[addr] {
			continue
		}
		seen[addr] = true
		out = append(out, addr)
	}
	return out
}

func resolveEndpoint(cfg Config) (host, port, addr string, timeout time.Duration, err error) {
	host = strings.TrimSpace(cfg.Host)
	if host == "" {
		return "", "", "", 0, fmt.Errorf("smtp: host required")
	}
	port = strings.TrimSpace(cfg.Port)
	if port == "" {
		if cfg.TLS {
			port = "465"
		} else {
			port = "587"
		}
	}
	timeout = cfg.Timeout
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	return host, port, net.JoinHostPort(host, port), timeout, nil
}

func implicitTLS(tlsFlag bool, port string) bool {
	return tlsFlag || port == "465"
}

func authOf(cfg Config, host string) smtp.Auth {
	if strings.TrimSpace(cfg.Username) == "" {
		return nil
	}
	return smtp.PlainAuth("", cfg.Username, cfg.Password, host)
}

func openClient(host, addr string, useTLS, startTLS bool, timeout time.Duration) (*smtp.Client, func(), error) {
	deadline := time.Now().Add(timeout)
	var conn net.Conn
	var err error
	if useTLS {
		dialer := &net.Dialer{Timeout: timeout}
		conn, err = tls.DialWithDialer(dialer, "tcp", addr, &tls.Config{ServerName: host})
	} else {
		dialer := net.Dialer{Timeout: timeout}
		conn, err = dialer.Dial("tcp", addr)
	}
	if err != nil {
		return nil, nil, err
	}
	_ = conn.SetDeadline(deadline)
	c, err := smtp.NewClient(conn, host)
	if err != nil {
		_ = conn.Close()
		return nil, nil, err
	}
	closer := func() {
		_ = c.Close()
		_ = conn.Close()
	}
	if !useTLS && startTLS {
		if ok, _ := c.Extension("STARTTLS"); ok {
			if err := c.StartTLS(&tls.Config{ServerName: host}); err != nil {
				closer()
				return nil, nil, err
			}
		}
	}
	return c, closer, nil
}

func finish(c *smtp.Client, auth smtp.Auth, from string, to []string, msg []byte) error {
	if auth != nil {
		if err := c.Auth(auth); err != nil {
			return err
		}
	}
	if err := c.Mail(from); err != nil {
		return err
	}
	for _, addr := range to {
		if err := c.Rcpt(addr); err != nil {
			return err
		}
	}
	w, err := c.Data()
	if err != nil {
		return err
	}
	if _, err := w.Write(msg); err != nil {
		_ = w.Close()
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	return c.Quit()
}

func buildMessage(cfg Config, from string, to []string, subject, text, html string) []byte {
	fromHdr := from
	if name := strings.TrimSpace(cfg.FromName); name != "" {
		fromHdr = fmt.Sprintf("%s <%s>", name, from)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "From: %s\r\n", fromHdr)
	fmt.Fprintf(&b, "To: %s\r\n", strings.Join(to, ", "))
	fmt.Fprintf(&b, "Subject: %s\r\n", encodeHeader(subject))
	b.WriteString("MIME-Version: 1.0\r\n")
	html = strings.TrimSpace(html)
	if html == "" {
		b.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
		b.WriteString("\r\n")
		writeBody(&b, text)
		return []byte(b.String())
	}
	boundary := "----=_ap_" + strings.ReplaceAll(fmt.Sprintf("%d", time.Now().UnixNano()), "-", "")
	fmt.Fprintf(&b, "Content-Type: multipart/alternative; boundary=\"%s\"\r\n", boundary)
	b.WriteString("\r\n")
	fmt.Fprintf(&b, "--%s\r\n", boundary)
	b.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
	b.WriteString("\r\n")
	writeBody(&b, text)
	fmt.Fprintf(&b, "--%s\r\n", boundary)
	b.WriteString("Content-Type: text/html; charset=UTF-8\r\n")
	b.WriteString("\r\n")
	writeBody(&b, html)
	fmt.Fprintf(&b, "--%s--\r\n", boundary)
	return []byte(b.String())
}

func writeBody(b *strings.Builder, body string) {
	b.WriteString(body)
	if !strings.HasSuffix(body, "\n") {
		b.WriteString("\n")
	}
}

func sanitizeHeader(s string) string {
	s = strings.ReplaceAll(s, "\r", " ")
	return strings.ReplaceAll(s, "\n", " ")
}

func encodeHeader(s string) string {
	s = sanitizeHeader(s)
	for _, r := range s {
		if r > 127 {
			return mime.QEncoding.Encode("utf-8", s)
		}
	}
	return s
}

func asString(v any) string {
	if v == nil {
		return ""
	}
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
		return strings.TrimSpace(fmt.Sprint(t))
	}
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

func asBool(v any) bool {
	switch t := v.(type) {
	case bool:
		return t
	case string:
		s := strings.ToLower(strings.TrimSpace(t))
		return s == "1" || s == "true" || s == "yes"
	default:
		return false
	}
}

func asBoolDefault(v any, fallback bool) bool {
	if v == nil {
		return fallback
	}
	if s, ok := v.(string); ok && strings.TrimSpace(s) == "" {
		return fallback
	}
	return asBool(v)
}
