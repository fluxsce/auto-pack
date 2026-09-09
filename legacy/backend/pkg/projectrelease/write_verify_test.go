package projectrelease

import (
	"os"
	"path/filepath"
	"testing"
)

func TestVerifyWrittenReleaseFiles_pomVersion(t *testing.T) {
	dir := t.TempDir()
	pom := filepath.Join(dir, "pom.xml")
	body := []byte(`<project><version>V9.0.0-P08-04-SNAPSHOT</version></project>`)
	if err := os.WriteFile(pom, body, 0o644); err != nil {
		t.Fatal(err)
	}
	var steps []string
	log := newPublishLogger(&steps)
	if err := verifyWrittenReleaseFiles(map[string][]byte{pom: body}, "V9.0.0-P08-04", log); err != nil {
		t.Fatal(err)
	}
	bad := []byte(`<project><version>V9.0.0-P08-01-SNAPSHOT</version></project>`)
	if err := os.WriteFile(pom, bad, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := verifyWrittenReleaseFiles(map[string][]byte{pom: body}, "V9.0.0-P08-04", log); err == nil {
		t.Fatal("expected verify failure when disk differs")
	}
}

func TestRelativeToWorkDir_linuxStyle(t *testing.T) {
	root := t.TempDir()
	pom := filepath.Join(root, "pom.xml")
	if err := os.WriteFile(pom, []byte("<project/>"), 0o644); err != nil {
		t.Fatal(err)
	}
	rel, err := relativeToWorkDir(root, pom)
	if err != nil {
		t.Fatal(err)
	}
	if rel != "pom.xml" {
		t.Fatalf("got %q", rel)
	}
}
