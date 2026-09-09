package smtp

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"io"
	"math/big"
	"net"
	"strings"
	"testing"
	"time"
)

func TestSplitTo(t *testing.T) {
	got := SplitTo("a@x.com, b@x.com; a@x.com")
	if len(got) != 2 || got[0] != "a@x.com" || got[1] != "b@x.com" {
		t.Fatal(got)
	}
}

func TestMapConfigAndSendRequiresTo(t *testing.T) {
	cfg := MapConfig(map[string]any{
		"host":        "mail.example",
		"from":        "pack@example.com",
		"starttls":    true,
		"timeout_sec": 15,
	})
	if cfg.Host != "mail.example" || cfg.From != "pack@example.com" || !cfg.StartTLS {
		t.Fatalf("%+v", cfg)
	}
	if err := Send(cfg, "s", "b"); err == nil || !strings.Contains(err.Error(), "no recipients") {
		t.Fatalf("err=%v", err)
	}
	cfg.To = "ops@example.com"
	if len(cfg.Recipients()) != 1 {
		t.Fatal(cfg.Recipients())
	}
}

func TestBuildMessage(t *testing.T) {
	msg := string(buildMessage(Config{FromName: "Auto Pack"}, "a@x.com", []string{"b@x.com"}, "hi", "body", ""))
	if !strings.Contains(msg, "From: Auto Pack <a@x.com>") || !strings.Contains(msg, "Subject: hi") {
		t.Fatal(msg)
	}
	zh := string(buildMessage(Config{}, "a@x.com", []string{"b@x.com"}, "[auto-pack] 工程 已成功", "工程名称：x", ""))
	if !strings.Contains(zh, "=?utf-8?q?") && !strings.Contains(zh, "=?UTF-8?q?") && !strings.Contains(zh, "=?utf-8?Q?") {
		t.Fatalf("subject should be rfc2047: %s", zh)
	}
	html := string(buildMessage(Config{}, "a@x.com", []string{"b@x.com"}, "hi", "plain", "<p>html</p>"))
	if !strings.Contains(html, "multipart/alternative") || !strings.Contains(html, "text/html") || !strings.Contains(html, "<p>html</p>") {
		t.Fatal(html)
	}
}

func TestCheckDoesNotRequireTo(t *testing.T) {
	err := Check(context.Background(), Config{Host: "127.0.0.1", Port: "1", Timeout: 200 * time.Millisecond})
	if err == nil {
		t.Fatal("expected dial error")
	}
	if strings.Contains(err.Error(), "no recipients") {
		t.Fatal(err)
	}
}

func TestImplicitTLS(t *testing.T) {
	if !implicitTLS(false, "465") || implicitTLS(false, "587") || !implicitTLS(true, "587") {
		t.Fatal("implicit TLS rules")
	}
	if got := (Config{Host: "smtp.exmail.qq.com", Port: "465"}).Addr(); got != "smtp.exmail.qq.com:465" {
		t.Fatal(got)
	}
}

func TestCheckPlainSMTP(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	errCh := make(chan error, 1)
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			errCh <- err
			return
		}
		defer conn.Close()
		errCh <- serveSMTP(conn)
	}()
	_, port, err := net.SplitHostPort(ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	err = Check(context.Background(), Config{
		Host:     "127.0.0.1",
		Port:     port,
		Username: "u",
		Password: "p",
		StartTLS: false,
		Timeout:  2 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := <-errCh; err != nil {
		t.Fatal(err)
	}
}

func TestCheckPort465UsesTLS(t *testing.T) {
	ln, err := tls.Listen("tcp", "127.0.0.1:0", &tls.Config{Certificates: []tls.Certificate{selfSigned(t)}})
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		_ = conn.SetDeadline(time.Now().Add(2 * time.Second))
		buf := make([]byte, 32)
		_, _ = conn.Read(buf)
	}()
	_, port, err := net.SplitHostPort(ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	started := time.Now()
	err = Check(context.Background(), Config{Host: "127.0.0.1", Port: port, TLS: true, Timeout: time.Second})
	if err == nil {
		t.Fatal("expected tls handshake/verify error")
	}
	if strings.Contains(err.Error(), "no recipients") {
		t.Fatal(err)
	}
	if time.Since(started) > 3*time.Second {
		t.Fatalf("tls path hung: %v", err)
	}
}

func TestCheckPlaintextOnTLSDeadline(t *testing.T) {
	ln, err := tls.Listen("tcp", "127.0.0.1:0", &tls.Config{Certificates: []tls.Certificate{selfSigned(t)}})
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		time.Sleep(2 * time.Second)
	}()
	_, port, err := net.SplitHostPort(ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	started := time.Now()
	err = Check(context.Background(), Config{Host: "127.0.0.1", Port: port, StartTLS: false, Timeout: 300 * time.Millisecond})
	if err == nil {
		t.Fatal("expected timeout talking plaintext to tls")
	}
	if time.Since(started) > 2*time.Second {
		t.Fatalf("deadline not applied: %v", err)
	}
}

func serveSMTP(conn net.Conn) error {
	br := bufio.NewReader(conn)
	if _, err := io.WriteString(conn, "220 test ESMTP\r\n"); err != nil {
		return err
	}
	for {
		line, err := br.ReadString('\n')
		if err != nil {
			return err
		}
		upper := strings.ToUpper(strings.TrimSpace(line))
		switch {
		case strings.HasPrefix(upper, "EHLO"), strings.HasPrefix(upper, "HELO"):
			if _, err := io.WriteString(conn, "250-localhost\r\n250 AUTH PLAIN\r\n"); err != nil {
				return err
			}
		case strings.HasPrefix(upper, "AUTH"):
			if _, err := io.WriteString(conn, "235 2.7.0 OK\r\n"); err != nil {
				return err
			}
		case strings.HasPrefix(upper, "QUIT"):
			_, err := io.WriteString(conn, "221 bye\r\n")
			return err
		default:
			if _, err := io.WriteString(conn, "250 OK\r\n"); err != nil {
				return err
			}
		}
	}
}

func selfSigned(t *testing.T) tls.Certificate {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "127.0.0.1"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}
}
