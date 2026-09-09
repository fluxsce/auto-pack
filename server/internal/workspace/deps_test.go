package workspace

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMavenRootPrefersWorkdir(t *testing.T) {
	base := t.TempDir()
	if err := os.MkdirAll(filepath.Join(base, "app"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(base, "pom.xml"), []byte("<project/>"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(base, "app", "pom.xml"), []byte("<project/>"), 0o644); err != nil {
		t.Fatal(err)
	}
	abs, src, ok := mavenRoot(base, "app", []string{"pom.xml", "app/pom.xml"})
	if !ok || src != "app/pom.xml" || filepath.Base(abs) != "app" {
		t.Fatalf("abs=%s src=%s ok=%v", abs, src, ok)
	}
}

func TestParsePackageJSONAndGoMod(t *testing.T) {
	npm := parsePackageJSON("package.json", `{
		"dependencies": {"vue": "^3.5.0"},
		"devDependencies": {"vite": "^7.0.0"}
	}`)
	if len(npm) != 2 {
		t.Fatalf("npm=%v", npm)
	}
	gomod := parseGoMod("go.mod", `
module example.com/app

go 1.22

require (
	github.com/foo/bar v1.2.3
)
`)
	if len(gomod) != 1 || gomod[0].Name != "github.com/foo/bar" || gomod[0].Version != "v1.2.3" {
		t.Fatalf("go.mod=%v", gomod)
	}
}
