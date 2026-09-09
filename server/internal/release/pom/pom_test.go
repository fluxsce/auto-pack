package pom

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPomVersionForTag(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"V9.0.0-P08", "9.0.0-P08-SNAPSHOT"},
		{"v9.0.0-P08-03", "9.0.0-P08-03-SNAPSHOT"},
		{"9.0.0-P08", "9.0.0-P08-SNAPSHOT"},
		{"V9.0.0-P08-SNAPSHOT", "9.0.0-P08-SNAPSHOT"},
		{"REV-1.0", "REV-1.0-SNAPSHOT"},
	}
	for _, c := range cases {
		if got := PomVersionForTag(c.in); got != c.want {
			t.Fatalf("%s: got %s want %s", c.in, got, c.want)
		}
	}
}

func TestCollectPomVersionUpdatesRewritesProjectNotOnlyParent(t *testing.T) {
	dir := t.TempDir()
	root := `<?xml version="1.0"?>
<project>
  <modelVersion>4.0.0</modelVersion>
  <parent>
    <groupId>com.flux</groupId>
    <artifactId>datahub-utils-parentPom</artifactId>
    <version>V8.0.0</version>
  </parent>
  <artifactId>flux-collaboration-utils</artifactId>
  <version>V8.0.0</version>
  <properties>
    <datahub.core.version>V8.0.0</datahub.core.version>
  </properties>
</project>
`
	child := `<?xml version="1.0"?>
<project>
  <modelVersion>4.0.0</modelVersion>
  <parent>
    <groupId>com.flux</groupId>
    <artifactId>flux-collaboration-utils</artifactId>
    <version>V8.0.0</version>
  </parent>
  <artifactId>flux-collaboration-utils-core</artifactId>
</project>
`
	if err := os.WriteFile(filepath.Join(dir, "pom.xml"), []byte(root), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "core"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "core", "pom.xml"), []byte(child), 0o644); err != nil {
		t.Fatal(err)
	}
	files, steps, err := CollectPomVersionUpdates(dir, "V9.0.0-P01")
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 {
		t.Fatalf("files=%d steps=%v", len(files), steps)
	}
	rootOut := string(files[filepath.Join(dir, "pom.xml")])
	if !strings.Contains(rootOut, "<artifactId>flux-collaboration-utils</artifactId>\n  <version>9.0.0-P01-SNAPSHOT</version>") {
		t.Fatalf("project version not updated\n%s", rootOut)
	}
	if !strings.Contains(rootOut, "<artifactId>datahub-utils-parentPom</artifactId>\n    <version>9.0.0-P01-SNAPSHOT</version>") {
		t.Fatalf("known parent version not updated\n%s", rootOut)
	}
	if !strings.Contains(rootOut, "<datahub.core.version>9.0.0-P01-SNAPSHOT</datahub.core.version>") {
		t.Fatalf("datahub.core.version not updated\n%s", rootOut)
	}
	childOut := string(files[filepath.Join(dir, "core", "pom.xml")])
	if !strings.Contains(childOut, "<artifactId>flux-collaboration-utils</artifactId>\n    <version>9.0.0-P01-SNAPSHOT</version>") {
		t.Fatalf("module parent version not updated\n%s", childOut)
	}
}
