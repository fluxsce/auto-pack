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
		Text:    "text",
		HTML:    "<p>html</p>",
	}
	raw, err := buildMIME(cfg, msg)
	if err != nil {
		t.Fatal(err)
	}
	s := string(raw)
	if !strings.Contains(s, "multipart/alternative") {
		t.Fatalf("want alternative multipart:\n%s", s)
	}
	if !strings.Contains(s, "<p>html</p>") {
		t.Fatalf("missing html part:\n%s", s)
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
