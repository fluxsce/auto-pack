package workspace

import (
	"errors"
	"path/filepath"
	"testing"
)

func TestCheckSlug(t *testing.T) {
	ok := []string{"default", "src-copy", "App_01", "a", "foo.bar", "V9.0.0-P09"}
	for _, name := range ok {
		if err := CheckSlug(name); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
	}
	bad := []string{"", "../etc", "a/b", "foo bar", "工程", "V9..0", ".hidden", "V9.0.0-P09.", "-lead", "_lead", "CON", "com1", "a@b"}
	for _, name := range bad {
		if err := CheckSlug(name); !errors.Is(err, ErrInvalidSlug) {
			t.Fatalf("%s: want ErrInvalidSlug, got %v", name, err)
		}
	}
}

func TestCheckCodeAllowsVersion(t *testing.T) {
	ok := []string{"default", "V9.0.0-P09", "v1.2.3", "App_01"}
	for _, name := range ok {
		if err := CheckCode(name); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
	}
	bad := []string{"", "../etc", "V9..0", ".hidden", "V9.0.0-P09.", "-lead", "foo bar", "工程"}
	for _, name := range bad {
		if err := CheckCode(name); !errors.Is(err, ErrInvalidCode) {
			t.Fatalf("%s: want ErrInvalidCode, got %v", name, err)
		}
	}
	if Slug("V9.0.0-P09", "id1") != "V9.0.0-P09" {
		t.Fatalf("slug=%s", Slug("V9.0.0-P09", "id1"))
	}
}

func TestCheckDisplayName(t *testing.T) {
	if err := CheckDisplayName("默认工作空间"); err != nil {
		t.Fatal(err)
	}
	if err := CheckDisplayName("demo / prod"); !errors.Is(err, ErrInvalidName) {
		t.Fatalf("slash: %v", err)
	}
	if err := CheckDisplayName(""); err == nil {
		t.Fatal("empty")
	}
}

func TestCheckRelPath(t *testing.T) {
	if err := CheckRelPath(""); err != nil {
		t.Fatal(err)
	}
	if err := CheckRelPath("demo-app"); err != nil {
		t.Fatal(err)
	}
	if err := CheckRelPath("pkg/demo-app"); err != nil {
		t.Fatal(err)
	}
	if err := CheckRelPath("V9.0.0-P09"); err != nil {
		t.Fatal(err)
	}
	if err := CheckRelPath("../secret"); !errors.Is(err, ErrInvalidSlug) {
		t.Fatalf("got %v", err)
	}
}

func TestProjectDirJail(t *testing.T) {
	root := t.TempDir()
	got := ProjectDir(root, "demo", "id1", "../secret")
	want := filepath.Join(root, "demo", "secret")
	if got != want {
		t.Fatalf("got %s want %s", got, want)
	}
	if !UnderRoot(root, got) {
		t.Fatal(got)
	}
}
