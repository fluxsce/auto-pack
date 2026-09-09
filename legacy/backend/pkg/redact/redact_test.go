package redact

import (
	"strings"
	"testing"
)

func TestURL(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"https://user:secret@git.example.com/repo.git", "https://****@git.example.com/repo.git"},
		{"https://oauth2:secret@git.example.com/a/b.git", "https://****@git.example.com/a/b.git"},
		{"https://git.example.com/repo.git", "https://git.example.com/repo.git"},
	}
	for _, tc := range tests {
		if got := URL(tc.in); got != tc.want {
			t.Fatalf("URL(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestRedact(t *testing.T) {
	in := "fatal: unable to access 'https://oauth2:token@git.example.com/x.git': 401"
	got := Redact(in)
	if strings.Contains(got, "token") || strings.Contains(got, "oauth2:") {
		t.Fatalf("got %q", got)
	}
	if !strings.Contains(got, "****@git.example.com") {
		t.Fatalf("got %q", got)
	}
}

func TestCommandArgs(t *testing.T) {
	args := CommandArgs([]string{"ls-remote", "https://oauth2:secret@host/a.git", "main"})
	if args[1] != "https://****@host/a.git" {
		t.Fatalf("got %v", args)
	}
}
