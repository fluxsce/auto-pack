package env

import (
	"bufio"
	"context"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRunProbeRejectsEmptySMTPHost(t *testing.T) {
	res := runProbe(context.Background(), "smtp", map[string]any{}, "")
	if res.OK || res.Message != "host required" {
		t.Fatalf("%+v", res)
	}
}

func TestRunProbeRejectsEmptyOSSEndpoint(t *testing.T) {
	res := runProbe(context.Background(), "oss", map[string]any{"bucket": "b"}, "")
	if res.OK || res.Message != "endpoint required" {
		t.Fatalf("%+v", res)
	}
}

func TestRunProbeOSSRequiresSecrets(t *testing.T) {
	res := runProbe(context.Background(), "oss", map[string]any{
		"endpoint": "https://oss-cn-hangzhou.aliyuncs.com",
		"bucket":   "datahub",
	}, "")
	if res.OK || !strings.Contains(res.Message, "access_key") {
		t.Fatalf("%+v", res)
	}
}

func TestRunProbeRejectsEmptyMNSEndpoint(t *testing.T) {
	res := runProbe(context.Background(), "mq", map[string]any{
		"request_queue": "REQ",
		"result_queue":  "RES",
	}, "")
	if res.OK || res.Message != "endpoint required" {
		t.Fatalf("%+v", res)
	}
}

func TestRunProbeRejectsEmptyMNSQueues(t *testing.T) {
	res := runProbe(context.Background(), "mq", map[string]any{"endpoint": "https://mns.example"}, "")
	if res.OK || res.Message != "request_queue required" {
		t.Fatalf("%+v", res)
	}
	res = runProbe(context.Background(), "mq", map[string]any{
		"endpoint":      "https://mns.example",
		"request_queue": "REQ",
	}, "")
	if res.OK || res.Message != "result_queue required" {
		t.Fatalf("%+v", res)
	}
}

func TestRunProbeRejectsEmptySSHHost(t *testing.T) {
	res := runProbe(context.Background(), "ssh", map[string]any{}, "")
	if res.OK {
		t.Fatal("empty host must fail")
	}
}

func TestRunProbeJDKHome(t *testing.T) {
	dir := t.TempDir()
	res := runProbe(context.Background(), "jdk", map[string]any{"java_home": dir}, "")
	if !res.OK {
		t.Fatalf("java_home dir should pass stat: %s", res.Message)
	}
}

func TestRunProbeJDKMissingHome(t *testing.T) {
	res := runProbe(context.Background(), "jdk", map[string]any{"java_home": filepath.Join(t.TempDir(), "missing")}, "")
	if res.OK {
		t.Fatal("missing home must fail")
	}
}

func TestResolveToolchainBinFromNodeHome(t *testing.T) {
	root := t.TempDir()
	bin := filepath.Join(root, "bin")
	if err := os.Mkdir(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	node := filepath.Join(bin, toolchainExeNames("node")[0])
	if err := os.WriteFile(node, []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	if got := resolveToolchainBin(bin, "node"); got != node {
		t.Fatalf("bin dir: got %s want %s", got, node)
	}
	if got := resolveToolchainBin(root, "node"); got != node {
		t.Fatalf("home dir: got %s want %s", got, node)
	}
	if got := resolveToolchainBin(node, "node"); got != node {
		t.Fatalf("file: got %s want %s", got, node)
	}
}

func TestProbeBinaryRejectsDirectory(t *testing.T) {
	dir := t.TempDir()
	res := probeBinary(context.Background(), dir, "-v")
	if res.OK {
		t.Fatal("directory must fail")
	}
	if !strings.Contains(res.Message, "directory") {
		t.Fatalf("want directory hint, got %s", res.Message)
	}
}

func TestRunProbeGitBinary(t *testing.T) {
	res := runProbe(context.Background(), "git", map[string]any{"git_path": "git"}, "")
	if res.Message == "" {
		t.Fatal("expected message")
	}
}

func TestCompleteTestRecipientAddsDomain(t *testing.T) {
	got := completeTestRecipient("shangjian")
	if got != "shangjian@flux.com.cn" {
		t.Fatal(got)
	}
}

func TestRunProbeSMTPRequiresTestTo(t *testing.T) {
	res := runProbe(context.Background(), "smtp", map[string]any{"host": "smtp.example.com"}, "")
	if res.OK || res.Message != "test recipient required" {
		t.Fatalf("%+v", res)
	}
}

func TestRunProbeSMTPSendsTestMail(t *testing.T) {
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
		errCh <- acceptTestMail(conn)
	}()
	_, port, err := net.SplitHostPort(ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	res := runProbe(context.Background(), "smtp", map[string]any{
		"host":        "127.0.0.1",
		"port":        port,
		"from":        "pack@example.com",
		"starttls":    false,
		"timeout_sec": 2,
	}, "ops@example.com")
	if !res.OK {
		t.Fatal(res.Message)
	}
	if res.Details["to"] != "ops@example.com" {
		t.Fatalf("%+v", res.Details)
	}
	select {
	case err := <-errCh:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("server hung")
	}
}

func acceptTestMail(conn net.Conn) error {
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
			if _, err := io.WriteString(conn, "250-localhost\r\n250 OK\r\n"); err != nil {
				return err
			}
		case strings.HasPrefix(upper, "MAIL"), strings.HasPrefix(upper, "RCPT"):
			if _, err := io.WriteString(conn, "250 OK\r\n"); err != nil {
				return err
			}
		case strings.HasPrefix(upper, "DATA"):
			if _, err := io.WriteString(conn, "354 go\r\n"); err != nil {
				return err
			}
			for {
				body, err := br.ReadString('\n')
				if err != nil {
					return err
				}
				if strings.TrimRight(body, "\r\n") == "." {
					break
				}
			}
			if _, err := io.WriteString(conn, "250 OK\r\n"); err != nil {
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
