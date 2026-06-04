package packrun

import (
	"testing"
)

func TestSanitizeURLForLog(t *testing.T) {
	got := sanitizeURLForLog("https://user:secret@git.example.com/repo.git")
	want := "https://****@git.example.com/repo.git"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}
