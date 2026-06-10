package maven

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInjectSettingsIntoScript(t *testing.T) {
	settings := filepath.Join(t.TempDir(), "settings.xml")
	if err := os.WriteFile(settings, []byte("<settings/>"), 0o644); err != nil {
		t.Fatal(err)
	}

	got := InjectSettingsIntoScript("mvn clean package -DskipTests", settings)
	if !strings.Contains(got, "-s") || !strings.Contains(got, "settings.xml") {
		t.Fatalf("inject failed: %q", got)
	}

	unchanged := InjectSettingsIntoScript("mvn -s /other/settings.xml package", settings)
	if unchanged != "mvn -s /other/settings.xml package" {
		t.Fatalf("should not override existing -s: %q", unchanged)
	}
}

func TestSettingsCandidates_userM2(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	path := filepath.Join(home, ".m2", "settings.xml")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("<settings/>"), 0o644); err != nil {
		t.Fatal(err)
	}
	cands := SettingsCandidates("")
	found := false
	for _, c := range cands {
		if c == path {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected %q in candidates: %v", path, cands)
	}
}
