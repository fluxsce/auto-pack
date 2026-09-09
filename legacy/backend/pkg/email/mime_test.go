package email

import (
	"strings"
	"testing"
)

func TestBuildMIMEPlain(t *testing.T) {
	cfg := Config{Host: "smtp.example.com", From: "sender@example.com", FromName: "Auto Pack"}
	msg := Message{
		To:      []string{"user@example.com"},
		Subject: "Hello",
		Text:    "plain body",
	}
	raw, err := buildMIME(cfg, msg)
	if err != nil {
		t.Fatal(err)
	}
	s := string(raw)
	for _, want := range []string{
		"sender@example.com",
		"Auto Pack",
		"To: user@example.com",
		"Subject:",
		"plain body",
		"text/plain",
	} {
		if !strings.Contains(s, want) {
			t.Fatalf("missing %q in:\n%s", want, s)
		}
	}
}

func TestBuildMIMEAlternative(t *testing.T) {
	cfg := Config{Host: "smtp.example.com", From: "sender@example.com"}
	msg := Message{
		To:      []string{"user@example.com"},
		Subject: "Hi",
		Text:    "text line ending without newline",
		HTML:    "<p>html</p>",
	}
	raw, err := buildMIME(cfg, msg)
	if err != nil {
		t.Fatal(err)
	}
	s := string(raw)
	if !strings.Contains(s, `multipart/alternative; boundary="alt-`) {
		t.Fatalf("want quoted multipart boundary:\n%s", s)
	}
	if !strings.Contains(s, "<p>html</p>") {
		t.Fatalf("missing html part:\n%s", s)
	}
	// Regression: missing CRLF before next boundary glues text into "--alt-..."
	if strings.Contains(s, "newline--alt-") || strings.Contains(s, "newline--alt") {
		t.Fatalf("text part not terminated before boundary:\n%s", s)
	}
	if !strings.Contains(s, "newline\r\n--alt-") && !strings.Contains(s, "newline\n--alt-") {
		// quoted-printable may soft-wrap; ensure a boundary line still exists on its own
		idx := strings.Index(s, "Content-Type: text/html")
		if idx <= 0 {
			t.Fatalf("missing html content-type after text part:\n%s", s)
		}
		before := s[:idx]
		if !strings.Contains(before, "\r\n--alt-") && !strings.Contains(before, "\n--alt-") {
			t.Fatalf("boundary not on its own line before html part:\n%s", s)
		}
	}
}

func TestMessageValidate(t *testing.T) {
	err := Message{Subject: "x", Text: "y"}.Validate()
	if err == nil {
		t.Fatal("expected missing To")
	}
	err = Message{To: []string{"a@b.com"}, Subject: "x"}.Validate()
	if err == nil {
		t.Fatal("expected missing body")
	}
}
