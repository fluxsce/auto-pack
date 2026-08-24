package workspaceftppush

import (
	"archive/zip"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestFormatByteSize(t *testing.T) {
	cases := []struct {
		n    int64
		want string
	}{
		{500, "500 B"},
		{2048, "2.0 KiB"},
		{100 * 1024 * 1024, "100.0 MiB"},
		{2 * 1024 * 1024 * 1024, "2.0 GiB"},
	}
	for _, tc := range cases {
		if got := formatByteSize(tc.n); got != tc.want {
			t.Fatalf("formatByteSize(%d)=%q want %q", tc.n, got, tc.want)
		}
	}
}

func TestIsLikelyCompressed(t *testing.T) {
	if !isLikelyCompressed("lib/foo.jar") {
		t.Fatal("expected .jar compressed")
	}
	if isLikelyCompressed("readme.txt") {
		t.Fatal("expected .txt not compressed")
	}
}

func TestZipDirToTempUsesStoreForJar(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, "shared-datahub")
	if err := os.MkdirAll(filepath.Join(root, "lib"), 0o755); err != nil {
		t.Fatal(err)
	}
	jarPath := filepath.Join(root, "lib", "a.jar")
	if err := os.WriteFile(jarPath, []byte("jar-bytes"), 0o644); err != nil {
		t.Fatal(err)
	}
	txtPath := filepath.Join(root, "readme.txt")
	if err := os.WriteFile(txtPath, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}

	tmp, name, cleanup, err := zipDirToTemp(root)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	if name != "shared-datahub.zip" {
		t.Fatalf("zip name=%q", name)
	}

	zr, err := zip.OpenReader(tmp)
	if err != nil {
		t.Fatal(err)
	}
	defer zr.Close()

	methods := map[string]uint16{}
	for _, f := range zr.File {
		methods[f.Name] = f.Method
	}
	if methods["shared-datahub/lib/a.jar"] != zip.Store {
		t.Fatalf("jar method=%d want Store", methods["shared-datahub/lib/a.jar"])
	}
	if methods["shared-datahub/readme.txt"] != zip.Deflate {
		t.Fatalf("txt method=%d want Deflate", methods["shared-datahub/readme.txt"])
	}
}

func TestZipSourcesToTempMergesVDatahub(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "mod-a", "sce-vdatahub")
	b := filepath.Join(dir, "mod-b", "sce-vdatahub")
	if err := os.MkdirAll(filepath.Join(a, "js"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(b, "css"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(a, "js", "a.js"), []byte("from-a"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(b, "css", "b.css"), []byte("from-b"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(a, "index.html"), []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(b, "index.html"), []byte("new"), 0o644); err != nil {
		t.Fatal(err)
	}

	tmp, name, cleanup, err := zipSourcesToTemp([]string{a, b}, DefaultVDatahubZipRoot, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	if name != "sce-vdatahub.zip" {
		t.Fatalf("zip name=%q", name)
	}

	zr, err := zip.OpenReader(tmp)
	if err != nil {
		t.Fatal(err)
	}
	defer zr.Close()

	contents := map[string]string{}
	for _, f := range zr.File {
		if f.FileInfo().IsDir() {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		b, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			t.Fatal(err)
		}
		contents[f.Name] = string(b)
	}
	if contents["sce-vdatahub/js/a.js"] != "from-a" {
		t.Fatalf("a.js=%q", contents["sce-vdatahub/js/a.js"])
	}
	if contents["sce-vdatahub/css/b.css"] != "from-b" {
		t.Fatalf("b.css=%q", contents["sce-vdatahub/css/b.css"])
	}
	if contents["sce-vdatahub/index.html"] != "new" {
		t.Fatalf("index.html=%q want later source", contents["sce-vdatahub/index.html"])
	}
}

func TestZipSourcesToTempKeepsOriginalDirNames(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "sce-vdatahub")
	b := filepath.Join(dir, "sce-vdatahub-web")
	if err := os.MkdirAll(a, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(b, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(a, "a.js"), []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(b, "b.js"), []byte("b"), 0o644); err != nil {
		t.Fatal(err)
	}

	tmp, name, cleanup, err := zipSourcesToTemp([]string{a, b}, DefaultVDatahubZipRoot, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	if name != "sce-vdatahub.zip" {
		t.Fatalf("zip name=%q", name)
	}

	zr, err := zip.OpenReader(tmp)
	if err != nil {
		t.Fatal(err)
	}
	defer zr.Close()

	names := map[string]bool{}
	for _, f := range zr.File {
		names[f.Name] = true
	}
	if !names["sce-vdatahub/a.js"] || !names["sce-vdatahub-web/b.js"] {
		t.Fatalf("entries=%v", names)
	}
	if names["sce-vdatahub/b.js"] {
		t.Fatal("sce-vdatahub-web contents should not flatten into sce-vdatahub/")
	}
}
