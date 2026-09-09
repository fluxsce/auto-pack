package projectrelease

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSelectNotesForRelease_P07Cutoff(t *testing.T) {
	root := filepath.Join("..", "..", "data", "workspaces", "V9.0.0-P01", "flux-collaboration-datahub01api")
	notesPath := filepath.Join(root, "src", "main", "resources", "updatenotes", "UpdateNotes.txt")
	versionPath := filepath.Join(root, "src", "main", "resources", "Version.txt")
	if _, err := os.Stat(notesPath); err != nil {
		t.Skip("testdata not present:", err)
	}
	notes, err := os.ReadFile(notesPath)
	if err != nil {
		t.Fatal(err)
	}
	ver, err := os.ReadFile(versionPath)
	if err != nil {
		t.Fatal(err)
	}
	blocks := ParseVersionFile(string(ver))
	if len(blocks) < 2 {
		t.Fatal("expected at least 2 version blocks")
	}
	prevYMD := dateToYMD(blocks[1].ReleaseDate) // P06 date
	entries := ParseUpdateNotes(string(notes))
	selected := SelectNotesForRelease(entries, prevYMD, "20260409")
	if len(selected) == 0 {
		t.Fatal("expected notes for P07 window")
	}
	first := selected[0].DateYMD
	if first < prevYMD {
		t.Fatalf("first note %s should be after %s", first, prevYMD)
	}
}

func TestBuildVersionContentPreservesOlderBlocks(t *testing.T) {
	root := filepath.Join("..", "..", "data", "workspaces", "V9.0.0-P01", "flux-collaboration-datahub01api")
	versionPath := filepath.Join(root, "src", "main", "resources", "Version.txt")
	updateNotesPath := filepath.Join(root, "src", "main", "resources", "updatenotes", "UpdateNotes.txt")
	if _, err := os.Stat(versionPath); err != nil {
		t.Skip("testdata not present:", err)
	}
	out, err := BuildVersionContent(versionPath, updateNotesPath, SyncOptions{
		Tag:         "V9.0.0-P99-TEST",
		ReleaseDate: "2099-01-01",
		AllowRepeat: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "V9.0.0-P99-TEST") {
		t.Fatal("missing new version header")
	}
	if !strings.Contains(out, "V9.0.0-P07") {
		t.Fatal("older blocks should remain")
	}
}
