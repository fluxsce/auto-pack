package utils

import (
	"archive/zip"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCleanRel(t *testing.T) {
	got, err := CleanRel("src/pom.xml")
	if err != nil || got != "src/pom.xml" {
		t.Fatalf("got %q %v", got, err)
	}
	if _, err := CleanRel("../secret"); err == nil {
		t.Fatal("expected jail")
	}
}

func TestListAndRead(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "src", "a.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	exists, entries, err := ListDir(root, "", 0)
	if err != nil || !exists || len(entries) != 1 || !entries[0].IsDir {
		t.Fatalf("list root: exists=%v n=%d err=%v", exists, len(entries), err)
	}
	exists, entries, err = ListDir(root, "src", 0)
	if err != nil || !exists || len(entries) != 1 || entries[0].Name != "a.txt" {
		t.Fatalf("list src: %+v %v", entries, err)
	}
	if entries[0].Size != 5 || entries[0].ModTime == "" {
		t.Fatalf("file meta: %+v", entries[0])
	}
	if _, err := time.Parse(time.RFC3339, entries[0].ModTime); err != nil {
		t.Fatalf("mod_time %q: %v", entries[0].ModTime, err)
	}
	text, truncated, err := ReadText(root, "src/a.txt", 0)
	if err != nil || truncated || text != "hello" {
		t.Fatalf("read=%q trunc=%v err=%v", text, truncated, err)
	}
	if _, _, err := ReadText(root, "../x", 0); err == nil {
		t.Fatal("expected jail")
	}
	if err := os.WriteFile(filepath.Join(root, "src", "bin.dat"), []byte{0xff, 0xfe, 0x00}, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := ReadText(root, "src/bin.dat", 0); err != ErrNotText {
		t.Fatalf("want ErrNotText, got %v", err)
	}
}

func TestClearDirKeepsRoot(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "sub", "b.txt"), []byte("b"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := ClearDir(root, ""); err != nil {
		t.Fatal(err)
	}
	st, err := os.Stat(root)
	if err != nil || !st.IsDir() {
		t.Fatalf("root gone: %v", err)
	}
	items, err := os.ReadDir(root)
	if err != nil || len(items) != 0 {
		t.Fatalf("want empty, got %v %v", items, err)
	}
	if err := ClearDir(root, "../x"); err == nil {
		t.Fatal("expected jail")
	}
}

func TestRemoveRel(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "a.txt")
	dir := filepath.Join(root, "sub")
	if err := os.WriteFile(file, []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "nested"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "nested", "b.txt"), []byte("b"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := RemoveRel(root, ""); err == nil {
		t.Fatal("expected refuse root")
	}
	if err := RemoveRel(root, "a.txt"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(file); !os.IsNotExist(err) {
		t.Fatalf("file still there: %v", err)
	}
	if err := RemoveRel(root, "sub"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("dir still there: %v", err)
	}
	st, err := os.Stat(root)
	if err != nil || !st.IsDir() {
		t.Fatalf("root gone: %v", err)
	}
	if err := RemoveRel(root, "../x"); err == nil {
		t.Fatal("expected jail")
	}
}

func TestFindRelSkipsTarget(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "app"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "target"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "pom.xml"), []byte("<project/>"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "app", "pom.xml"), []byte("<project/>"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "target", "pom.xml"), []byte("<skip/>"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := FindRel(root, []string{"pom.xml"}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("got %v", got)
	}
}

func TestZipFileAndDir(t *testing.T) {
	root := t.TempDir()
	src := filepath.Join(root, "n.txt")
	if err := os.WriteFile(src, []byte("n"), 0o644); err != nil {
		t.Fatal(err)
	}
	fileZip := filepath.Join(root, "one.zip")
	if err := ZipFile(src, fileZip); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(root, "pkg")
	if err := os.MkdirAll(filepath.Join(dir, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "sub", "b.txt"), []byte("b"), 0o644); err != nil {
		t.Fatal(err)
	}
	dirZip := filepath.Join(root, "dir.zip")
	if err := ZipDir(dir, dirZip); err != nil {
		t.Fatal(err)
	}
	r, err := zip.OpenReader(dirZip)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	if len(r.File) == 0 {
		t.Fatal("empty zip")
	}
	rootZip := filepath.Join(root, "rooted.zip")
	if err := ZipDirAs(dir, rootZip, "pkg", func(name string, isDir bool) bool {
		return isDir && strings.HasPrefix(name, ".")
	}); err != nil {
		t.Fatal(err)
	}
	rr, err := zip.OpenReader(rootZip)
	if err != nil {
		t.Fatal(err)
	}
	defer rr.Close()
	found := false
	for _, f := range rr.File {
		if f.Name == "pkg/sub/b.txt" {
			found = true
		}
	}
	if !found {
		t.Fatalf("missing rooted entry: %v", namesOf(rr))
	}
}

func TestZipSourcesKeepsOriginalDirNames(t *testing.T) {
	root := t.TempDir()
	a := filepath.Join(root, "sce-vdatahub-d12-web")
	b := filepath.Join(root, "sce-vdatahub-d13-web")
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
	dest := filepath.Join(root, "merged.zip")
	if err := ZipSources([]string{a, b}, dest, nil); err != nil {
		t.Fatal(err)
	}
	zr, err := zip.OpenReader(dest)
	if err != nil {
		t.Fatal(err)
	}
	defer zr.Close()
	names := map[string]bool{}
	for _, f := range zr.File {
		names[f.Name] = true
	}
	if !names["sce-vdatahub-d12-web/a.js"] || !names["sce-vdatahub-d13-web/b.js"] {
		t.Fatalf("entries=%v", names)
	}
}

func TestPushLayout(t *testing.T) {
	dir, upd := PushLayout("/deploy/update", "app.jar")
	if dir != "/deploy/update" || upd != "/deploy/update/app.jar" {
		t.Fatalf("dir=%q upd=%q", dir, upd)
	}
	dir, upd = PushLayout("/deploy/update/app.jar", "app.jar")
	if dir != "/deploy/update" || upd != "/deploy/update/app.jar" {
		t.Fatalf("file path dir=%q upd=%q", dir, upd)
	}
	dir, upd = PushLayout("deploy/update", "app.jar")
	if dir != "/deploy/update" || upd != "/deploy/update/app.jar" {
		t.Fatalf("rel dir=%q upd=%q", dir, upd)
	}
}

func TestDefaultBackupDir(t *testing.T) {
	ts := time.Date(2026, 9, 9, 16, 2, 3, 123456789, time.UTC)
	got := DefaultBackupDir("/deploy/update", ts)
	want := "/deploy/update/BAK/" + BackupDirStamp(ts)
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	if !strings.HasSuffix(got, "20260909160203123") {
		t.Fatalf("stamp %q", got)
	}
}

func namesOf(r *zip.ReadCloser) []string {
	out := make([]string, 0, len(r.File))
	for _, f := range r.File {
		out = append(out, f.Name)
	}
	return out
}
