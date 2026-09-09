package runner

import (
	"strings"
	"testing"
	"time"
)

func TestBuildUpdateNote(t *testing.T) {
	at := time.Date(2026, 9, 9, 16, 0, 0, 0, time.UTC)
	got := buildUpdateNote("alice", "fix login", "line1\nline2", []notePair{
		{backup: "/p/BAK/x/a.jar", update: "/p/a.jar"},
	}, at)
	want := "20260909 alice fix login\n备份包路径:/p/BAK/x/a.jar\n更新包路径:/p/a.jar\n    line1\n    line2\n"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestBuildUpdateNoteSkipsEmptyBackup(t *testing.T) {
	at := time.Date(2026, 9, 9, 16, 0, 0, 0, time.UTC)
	got := buildUpdateNote("bob", "new file", "", []notePair{
		{backup: "", update: "/p/b.jar"},
	}, at)
	if !strings.Contains(got, "备份包路径:\n") {
		t.Fatalf("empty backup line: %q", got)
	}
	if !strings.Contains(got, "更新包路径:/p/b.jar\n") {
		t.Fatalf("update line: %q", got)
	}
}

func TestMergeUpdateNote(t *testing.T) {
	got := mergeUpdateNote("old block\n", "new block\n")
	if got != "old block\n\nnew block\n" {
		t.Fatalf("%q", got)
	}
	if mergeUpdateNote("", "only\n") != "only\n" {
		t.Fatalf("empty existing")
	}
}

func TestFormatNoteDetail(t *testing.T) {
	got := formatNoteDetail("  a\n\n    already\n\tb")
	if got != "    a\n    already\n\tb" {
		t.Fatalf("%q", got)
	}
}
