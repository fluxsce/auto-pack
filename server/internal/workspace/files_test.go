package workspace

import (
	"testing"

	"auto-pack/server/internal/host/utils"
)

func TestSkipNestedGit(t *testing.T) {
	if !SkipNestedGit(".git", true) {
		t.Fatal("skip nested .git")
	}
	if SkipNestedGit(".git", false) {
		t.Fatal("do not skip .git file")
	}
	if SkipNestedGit("src", true) {
		t.Fatal("keep src")
	}
}

func TestCheckoutRelUsesUtilsJail(t *testing.T) {
	got, err := utils.CleanRel("src/pom.xml")
	if err != nil || got != "src/pom.xml" {
		t.Fatalf("got %q %v", got, err)
	}
	if _, err := utils.CleanRel("../secret"); err == nil {
		t.Fatal("expected jail")
	}
}
