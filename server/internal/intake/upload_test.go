package intake

import (
	"archive/zip"
	"context"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"auto-pack/server/internal/project"
)

func TestSelectPushFiles(t *testing.T) {
	work := t.TempDir()
	if err := os.MkdirAll(filepath.Join(work, "target"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"target/app.war", "target/other.war", "target/config.zip"} {
		if err := os.WriteFile(filepath.Join(work, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	all, err := selectPushFiles(work, nil)
	if err != nil || len(all) != 2 {
		t.Fatalf("wars=%v %v", all, err)
	}
	picked, err := selectPushFiles(work, []string{"config.zip", "app.war"})
	if err != nil || len(picked) != 2 {
		t.Fatalf("picked=%v %v", picked, err)
	}
	if filepath.Base(picked[0]) != "config.zip" || filepath.Base(picked[1]) != "app.war" {
		t.Fatalf("%v", picked)
	}
	if _, err := selectPushFiles(work, []string{"missing.jar"}); err == nil {
		t.Fatal("missing file must fail")
	}
}

func TestCollectOSSPushFilesZipsDirsAndMergesSce(t *testing.T) {
	work := t.TempDir()
	shared := filepath.Join(work, "shared-datahub")
	a := filepath.Join(work, "sce-vdatahub-d12-web")
	b := filepath.Join(work, "sce-vdatahub-d13-web")
	if err := os.MkdirAll(filepath.Join(shared, "lib"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(a, "js"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(b, "css"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(work, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{
		filepath.Join(work, "datahub01web.war"),
		filepath.Join(shared, "lib", "a.jar"),
		filepath.Join(a, "js", "a.js"),
		filepath.Join(b, "css", "b.css"),
		filepath.Join(work, ".git", "config"),
	} {
		if err := os.WriteFile(name, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	files, cleanup, err := collectOSSPushFiles(work)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	names := map[string]string{}
	for _, file := range files {
		names[filepath.Base(file)] = file
	}
	if _, ok := names["datahub01web.war"]; !ok {
		t.Fatalf("war missing: %v", files)
	}
	if _, ok := names["shared-datahub.zip"]; !ok {
		t.Fatalf("folder should be zipped: %v", files)
	}
	if _, ok := names["sce-vdatahub.zip"]; !ok {
		t.Fatalf("sce dirs should merge: %v", files)
	}
	if _, ok := names["sce-vdatahub-d12-web.zip"]; ok {
		t.Fatal("multiple sce dirs must not stay separate")
	}
	if _, ok := names[".git"]; ok {
		t.Fatal("hidden dir must be skipped")
	}
	inZip := zipNames(t, names["sce-vdatahub.zip"])
	if !inZip["sce-vdatahub-d12-web/js/a.js"] || !inZip["sce-vdatahub-d13-web/css/b.css"] {
		t.Fatalf("merged entries %v", inZip)
	}

	if err := os.RemoveAll(b); err != nil {
		t.Fatal(err)
	}
	single, singleCleanup, err := collectOSSPushFiles(work)
	if err != nil {
		t.Fatal(err)
	}
	defer singleCleanup()
	var singleNames []string
	for _, file := range single {
		singleNames = append(singleNames, filepath.Base(file))
	}
	sort.Strings(singleNames)
	want := []string{"datahub01web.war", "sce-vdatahub-d12-web.zip", "shared-datahub.zip"}
	if len(singleNames) != len(want) {
		t.Fatalf("single sce got %v", singleNames)
	}
	for i, name := range want {
		if singleNames[i] != name {
			t.Fatalf("single sce got %v", singleNames)
		}
	}
}

func TestOSSPushUsesResourceFilesWhenSet(t *testing.T) {
	svc, _, _, _ := setupIntake(t)
	ctx := context.Background()
	prj, err := svc.projects.GetRaw(ctx, "p-code")
	if err != nil {
		t.Fatal(err)
	}
	work, err := svc.projectWork(ctx, prj)
	if err != nil {
		t.Fatal(err)
	}
	shared := filepath.Join(work, "shared-datahub")
	if err := os.MkdirAll(shared, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(shared, "readme.txt"), []byte("lib"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(work, "extra.txt"), []byte("e"), 0o644); err != nil {
		t.Fatal(err)
	}
	var got []string
	svc.upload = func(_ context.Context, _ project.Project, local, _ string) error {
		got = append(got, filepath.Base(local))
		return nil
	}
	svc.sign = func(key string, _ time.Duration) (string, error) {
		return "https://signed.example/" + key, nil
	}
	n := sampleNotice()
	n.ResourceFiles = []string{"app.war"}
	_, remote, _, _, _, err := svc.pushProjectWars(ctx, prj, n)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != "app.war" || remote != "/开发/S01/app.war" {
		t.Fatalf("filtered uploaded %v remote %s", got, remote)
	}

	got = nil
	n.ResourceFiles = nil
	_, remote, _, _, _, err = svc.pushProjectWars(ctx, prj, n)
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(got)
	if len(got) != 3 || got[0] != "app.war" || got[1] != "extra.txt" || got[2] != "shared-datahub.zip" {
		t.Fatalf("blank resourceFiles uploaded %v remote %s", got, remote)
	}
	if !strings.Contains(remote, "/开发/S01/app.war") || !strings.Contains(remote, "/开发/S01/shared-datahub.zip") || !strings.Contains(remote, "/开发/S01/extra.txt") {
		t.Fatalf("remote %s", remote)
	}
}

func zipNames(t *testing.T, path string) map[string]bool {
	t.Helper()
	zr, err := zip.OpenReader(path)
	if err != nil {
		t.Fatal(err)
	}
	defer zr.Close()
	names := map[string]bool{}
	for _, file := range zr.File {
		names[file.Name] = true
	}
	return names
}

func TestPushFollowsOnePath(t *testing.T) {
	var dests []string
	svc := &Service{
		upload: func(_ context.Context, _ project.Project, _, dest string) error {
			dests = append(dests, dest)
			return nil
		},
		sign: func(string, time.Duration) (string, error) { return "https://signed.example/app.war", nil },
	}
	n := sampleNotice()
	n.FTPProjectRoot = "/ftp/S01/"
	_, remote, _, _, _, err := svc.pushWarFile(context.Background(), project.Project{}, "app.war", n, time.Now())
	if err != nil || remote != "/开发/S01/app.war" || len(dests) != 1 || dests[0] != "/开发/S01/app.war" {
		t.Fatalf("remote=%s dests=%v err=%v", remote, dests, err)
	}
	n.OSSProjectRoot = ""
	dests = nil
	_, remote, _, _, _, err = svc.pushWarFile(context.Background(), project.Project{}, "app.war", n, time.Now())
	if err != nil || remote != "/ftp/S01/app.war" || len(dests) != 1 || dests[0] != "/ftp/S01/app.war" {
		t.Fatalf("remote=%s dests=%v err=%v", remote, dests, err)
	}
	boom := errors.New("550 denied")
	svc.upload = func(context.Context, project.Project, string, string) error { return boom }
	_, _, _, _, _, err = svc.pushWarFile(context.Background(), project.Project{}, "app.war", n, time.Now())
	if err == nil || err.Error() != "FTP 550 denied" {
		t.Fatal(err)
	}
}

func TestOSSBackupKeyUsesDateClock(t *testing.T) {
	at := time.Date(2026, 10, 17, 0, 49, 0, 123000000, time.UTC)
	got := ossBackupKey("开发/S01/app.war", at)
	if got != "开发/S01/备份/20261017/084900/app.war" {
		t.Fatal(got)
	}
	root := ossBackupKey("app.war", at)
	if root != "备份/20261017/084900/app.war" {
		t.Fatal(root)
	}
}
