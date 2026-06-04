package projectcfg

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMatchArtifactFiles_warGlob(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "target")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	war := filepath.Join(target, "app.war")
	if err := os.WriteFile(war, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	other := filepath.Join(root, "readme.txt")
	if err := os.WriteFile(other, []byte("y"), 0o644); err != nil {
		t.Fatal(err)
	}

	matches, err := MatchArtifactFiles(root, "**/*.war")
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 1 || matches[0] != war {
		t.Fatalf("got %v want [%s]", matches, war)
	}
}

func TestRemoteRelPath_removePrefix(t *testing.T) {
	base := "/work"
	local := filepath.Join(base, "target", "app.war")
	got := RemoteRelPath(local, base, "target")
	if got != "app.war" {
		t.Fatalf("got %q want app.war", got)
	}
}
