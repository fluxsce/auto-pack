package projectcfg

import "testing"

func TestSlugFromProjectName(t *testing.T) {
	if got := SlugFromProjectName("Demo App"); got != "Demo-App" {
		t.Fatalf("Demo App: got %q", got)
	}
	if got := SlugFromProjectName("中文项目"); got == "" {
		t.Fatal("non-ascii name should produce hash slug")
	}
	if got := SlugFromProjectName(""); got != "" {
		t.Fatalf("empty: got %q", got)
	}
}

func TestSuggestWorkDirFromName(t *testing.T) {
	got := SuggestWorkDir("My Service")
	if got != "My-Service" {
		t.Fatalf("got %q", got)
	}
}
