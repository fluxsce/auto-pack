package projectrelease

import (
	"path/filepath"
	"testing"
)

func TestDeferPomToTargetBranch(t *testing.T) {
	cases := []struct {
		create         bool
		release, target string
		want           bool
	}{
		{false, "develop", "release/V9", false},
		{true, "develop", "", false},
		{true, "develop", "develop", false},
		{true, "develop", "Develop", false},
		{true, "develop", "release/V9.0.0-P08", true},
		{true, "release/V9", "release/V9-fix", true},
	}
	for _, tc := range cases {
		got := deferPomToTargetBranch(tc.create, tc.release, tc.target)
		if got != tc.want {
			t.Fatalf("create=%v release=%q target=%q: got %v want %v",
				tc.create, tc.release, tc.target, got, tc.want)
		}
	}
}

func TestSplitReleasePublishFiles(t *testing.T) {
	root := t.TempDir()
	version := filepath.Join(root, "src", "main", "resources", "Version.txt")
	rootPom := filepath.Join(root, "pom.xml")
	modPom := filepath.Join(root, "module-a", "pom.xml")
	other := filepath.Join(root, "README.md")

	files := map[string][]byte{
		version: []byte("V1"),
		rootPom: []byte("<project/>"),
		modPom:  []byte("<project/>"),
		other:   []byte("x"),
	}
	releaseFiles, pomFiles := splitReleasePublishFiles(files)
	if len(pomFiles) != 2 {
		t.Fatalf("pom files: got %d want 2", len(pomFiles))
	}
	if _, ok := pomFiles[rootPom]; !ok {
		t.Fatal("missing root pom")
	}
	if _, ok := pomFiles[modPom]; !ok {
		t.Fatal("missing module pom")
	}
	if len(releaseFiles) != 2 {
		t.Fatalf("release files: got %d want 2", len(releaseFiles))
	}
	if _, ok := releaseFiles[version]; !ok {
		t.Fatal("missing Version.txt")
	}
	if _, ok := releaseFiles[other]; !ok {
		t.Fatal("missing other non-pom file")
	}
}

func TestIsPomReleaseFile(t *testing.T) {
	if !isPomReleaseFile(filepath.Join("repo", "pom.xml")) {
		t.Fatal("root pom")
	}
	if !isPomReleaseFile(filepath.Join("repo", "module", "POM.XML")) {
		t.Fatal("case-insensitive pom")
	}
	if isPomReleaseFile(filepath.Join("repo", "Version.txt")) {
		t.Fatal("Version.txt is not pom")
	}
}
