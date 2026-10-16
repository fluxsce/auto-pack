package serverupdate

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestParsePackName(t *testing.T) {
	m, err := parsePackName("auto-pack-2.3.0-linux-amd64.tar.gz")
	if err != nil || m.version != "2.3.0" || m.goos != "linux" || m.goarch != "amd64" {
		t.Fatalf("%+v %v", m, err)
	}
	m, err = parsePackName("auto-pack-2.3.0-windows-amd64.zip")
	if err != nil || m.goos != "windows" {
		t.Fatalf("%+v %v", m, err)
	}
	if _, err := parsePackName("flux-guard-1.0.0-linux-amd64.tar.gz"); err == nil {
		t.Fatal("want error")
	}
}

func TestVersionLess(t *testing.T) {
	if !versionLess("2.2.9", "2.3.0") {
		t.Fatal("2.2.9 < 2.3.0")
	}
	if versionLess("2.3.0", "2.3.0") {
		t.Fatal("equal not less")
	}
	if !versionLess("dev", "1.0.0") {
		t.Fatal("dev should allow any")
	}
}

func TestMergeTreeKeepsConfigAndStagesBinary(t *testing.T) {
	root := t.TempDir()
	src := t.TempDir()
	_ = os.MkdirAll(filepath.Join(root, "configs"), 0o755)
	_ = os.MkdirAll(filepath.Join(root, "bin"), 0o755)
	_ = os.MkdirAll(filepath.Join(root, "web"), 0o755)
	_ = os.WriteFile(filepath.Join(root, "configs", "app.yaml"), []byte("keep: true\n"), 0o644)
	_ = os.WriteFile(filepath.Join(root, "bin", "autopack"), []byte("old-bin"), 0o755)
	_ = os.WriteFile(filepath.Join(root, "web", "index.html"), []byte("old"), 0o644)

	_ = os.MkdirAll(filepath.Join(src, "bin"), 0o755)
	_ = os.MkdirAll(filepath.Join(src, "web"), 0o755)
	_ = os.MkdirAll(filepath.Join(src, "configs"), 0o755)
	_ = os.MkdirAll(filepath.Join(src, "data"), 0o755)
	_ = os.WriteFile(filepath.Join(src, "bin", "autopack"), []byte("new-bin"), 0o755)
	_ = os.WriteFile(filepath.Join(src, "web", "index.html"), []byte("new"), 0o644)
	_ = os.WriteFile(filepath.Join(src, "configs", "app.yaml"), []byte("overwrite: true\n"), 0o644)
	_ = os.WriteFile(filepath.Join(src, "data", "skip.txt"), []byte("x"), 0o644)

	if err := mergeTree(src, root, binBase); err != nil {
		t.Fatal(err)
	}
	web, _ := os.ReadFile(filepath.Join(root, "web", "index.html"))
	if string(web) != "new" {
		t.Fatalf("web=%s", web)
	}
	cfg, _ := os.ReadFile(filepath.Join(root, "configs", "app.yaml"))
	if string(cfg) != "keep: true\n" {
		t.Fatalf("config overwritten: %s", cfg)
	}
	bin, _ := os.ReadFile(filepath.Join(root, "bin", "autopack"))
	if string(bin) != "old-bin" {
		t.Fatalf("running binary should not be overwritten: %s", bin)
	}
	if _, err := os.Stat(filepath.Join(root, "data", "skip.txt")); err == nil {
		t.Fatal("data/ should be skipped")
	}
}

func TestExtractAndPayloadRoot(t *testing.T) {
	dir := t.TempDir()
	archive := filepath.Join(dir, "auto-pack-9.9.9-"+runtime.GOOS+"-"+runtime.GOARCH+".tar.gz")
	if err := writeTestTar(archive); err != nil {
		t.Fatal(err)
	}
	sum := fileSHA(t, archive)
	if len(sum) != 64 {
		t.Fatal(sum)
	}
	extract := t.TempDir()
	if err := extractArchive(archive, extract, filepath.Base(archive)); err != nil {
		t.Fatal(err)
	}
	root, err := payloadRoot(extract)
	if err != nil {
		t.Fatal(err)
	}
	if findBinary(root, binBase) == "" {
		t.Fatalf("binary missing under %s", root)
	}
	if _, err := os.Stat(filepath.Join(root, "web", "index.html")); err != nil {
		t.Fatal(err)
	}
}

func TestExtractArchiveIgnoresTempSuffix(t *testing.T) {
	dir := t.TempDir()
	named := filepath.Join(dir, "auto-pack-9.9.9-linux-amd64.tar.gz")
	if err := writeTestTar(named); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(named)
	if err != nil {
		t.Fatal(err)
	}
	tmp, err := os.CreateTemp(dir, "ap-server-pack-*")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tmp.Write(raw); err != nil {
		t.Fatal(err)
	}
	if err := tmp.Close(); err != nil {
		t.Fatal(err)
	}
	extract := t.TempDir()
	if err := extractArchive(tmp.Name(), extract, "auto-pack-9.9.9-linux-amd64.tar.gz"); err != nil {
		t.Fatal(err)
	}
	root, err := payloadRoot(extract)
	if err != nil || findBinary(root, binBase) == "" {
		t.Fatalf("payload %s err %v", root, err)
	}
	bad := t.TempDir()
	if err := extractArchive(tmp.Name(), bad, "notes.txt"); err == nil {
		t.Fatal("want unsupported suffix")
	}
}

func writeTestTar(path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	gz := gzip.NewWriter(f)
	defer gz.Close()
	tw := tar.NewWriter(gz)
	defer tw.Close()
	files := map[string]string{
		"auto-pack/bin/autopack":   "bin",
		"auto-pack/web/index.html": "<html></html>",
		"auto-pack/VERSION":        "9.9.9",
	}
	for name, body := range files {
		hdr := &tar.Header{Name: name, Mode: 0o644, Size: int64(len(body))}
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		if _, err := io.WriteString(tw, body); err != nil {
			return err
		}
	}
	return nil
}

func fileSHA(t *testing.T, path string) string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}
