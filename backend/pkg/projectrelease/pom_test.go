package projectrelease

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPomVersionForTag(t *testing.T) {
	if got := PomVersionForTag("V10.0.0-P01"); got != "V10.0.0-P01-SNAPSHOT" {
		t.Fatalf("got %q", got)
	}
	if got := PomVersionForTag("V10.0.0-P01-SNAPSHOT"); got != "V10.0.0-P01-SNAPSHOT" {
		t.Fatalf("idempotent got %q", got)
	}
	if got := PomVersionForTag("  "); got != "" {
		t.Fatalf("empty got %q", got)
	}
}

func TestUpdatePomVersions_datahub(t *testing.T) {
	pomPath := filepath.Join(
		"..", "..", "data", "workspaces", "V9.0.0-P01", "flux-collaboration-datahub01api", "pom.xml",
	)
	raw, err := os.ReadFile(pomPath)
	if err != nil {
		t.Skip("testdata not present:", err)
	}
	tag := "V9.0.0-P08"
	pomVer := PomVersionForTag(tag)
	out, err := UpdatePomVersions(string(raw), tag)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "<version>"+pomVer+"</version>") {
		t.Fatal("expected project/parent version updated with SNAPSHOT")
	}
	if !strings.Contains(out, "<datahub.core.version>"+pomVer+"</datahub.core.version>") {
		t.Fatal("expected datahub.core.version inserted with SNAPSHOT")
	}
	if strings.Contains(out, "10.0.0-SNAPSHOT") {
		t.Fatal("old snapshot version should be replaced")
	}
}

func TestUpdatePomVersions_srvApi(t *testing.T) {
	pomPath := filepath.Join(
		"..", "..", "data", "workspaces", "V9.0.0-P01", "flux-collaboration-SrvApi", "pom.xml",
	)
	raw, err := os.ReadFile(pomPath)
	if err != nil {
		t.Skip("testdata not present:", err)
	}
	tag := "V9.0.0-P08"
	pomVer := PomVersionForTag(tag)
	out, err := UpdatePomVersions(string(raw), tag)
	if err != nil {
		t.Fatal(err)
	}
	count := strings.Count(out, "<version>"+pomVer+"</version>")
	if count < 2 {
		t.Fatalf("expected parent and project version, got %d", count)
	}
	if strings.Contains(out, "datahub.core.version") {
		t.Fatal("SrvApi pom should not get datahub.core.version")
	}
}

func TestUpdatePomVersions_onlyDatahubParentVersion(t *testing.T) {
	const pom = `<project>
<parent>
<groupId>com.flux</groupId>
<artifactId>other-parent</artifactId>
<version>1.0.0</version>
</parent>
<version>10.0.0-SNAPSHOT</version>
</project>`
	tag := "V9.0.0-P01-01"
	pomVer := PomVersionForTag(tag)
	out, skip, err := updatePomVersions(pom, tag, "")
	if err != nil || skip != "" {
		t.Fatalf("err=%v skip=%q", err, skip)
	}
	if !strings.Contains(out, "<version>1.0.0</version>") {
		t.Fatal("other parent version should stay unchanged")
	}
	if strings.Count(out, "<version>"+pomVer+"</version>") != 1 {
		t.Fatalf("only project version should update: %s", out)
	}
}

func TestUpdatePomVersions_datahubParentArtifact(t *testing.T) {
	const pom = `<project>
<parent>
<groupId>com.flux</groupId>
<artifactId>datahub-utils-parentPom</artifactId>
<version>9.0.0-P00</version>
</parent>
<version>10.0.0-SNAPSHOT</version>
</project>`
	tag := "V9.0.0-P01-01"
	pomVer := PomVersionForTag(tag)
	out, skip, err := updatePomVersions(pom, tag, "")
	if err != nil || skip != "" {
		t.Fatalf("err=%v skip=%q", err, skip)
	}
	if !strings.Contains(out, "<artifactId>datahub-utils-parentPom</artifactId>") {
		t.Fatal("parent block missing")
	}
	if strings.Count(out, "<version>"+pomVer+"</version>") != 2 {
		t.Fatalf("parent and project version should update: %s", out)
	}
	if strings.Contains(out, "9.0.0-P00") {
		t.Fatal("old parent version should be replaced")
	}
}

func TestUpdatePomVersions_noParentVersion(t *testing.T) {
	const pom = `<project>
<parent>
<groupId>com.flux</groupId>
<artifactId>datahub-utils-parentPom</artifactId>
</parent>
<version>10.0.0-SNAPSHOT</version>
</project>`
	tag := "V9.0.0-P01-01"
	pomVer := PomVersionForTag(tag)
	out, skip, err := updatePomVersions(pom, tag, "")
	if err != nil || skip != "" {
		t.Fatalf("err=%v skip=%q", err, skip)
	}
	if !strings.Contains(out, "<version>"+pomVer+"</version>") {
		t.Fatalf("got %s", out)
	}
}

func TestUpdatePomVersions_childModuleParentVersion(t *testing.T) {
	const pom = `<project>
<parent>
<groupId>com.flux.wmsv6</groupId>
<artifactId>flux-collaboration-storage-repeater-parent</artifactId>
<version>V10.0.0-P01</version>
</parent>
<artifactId>moonbox-tools</artifactId>
</project>`
	tag := "V10.0.0-P02"
	pomVer := PomVersionForTag(tag)
	out, skip, err := updatePomVersions(pom, tag, "flux-collaboration-storage-repeater-parent")
	if err != nil || skip != "" {
		t.Fatalf("err=%v skip=%q", err, skip)
	}
	if !strings.Contains(out, "<version>"+pomVer+"</version>") {
		t.Fatalf("parent version should update: %s", out)
	}
	if strings.Contains(out, "V10.0.0-P01") {
		t.Fatalf("old parent version should be replaced: %s", out)
	}
}

func TestCollectPomVersionUpdates_multiModule(t *testing.T) {
	dir := t.TempDir()
	rootPOM := `<project>
<groupId>com.flux.wmsv6</groupId>
<artifactId>flux-collaboration-storage-repeater-parent</artifactId>
<version>V10.0.0-P01-SNAPSHOT</version>
<packaging>pom</packaging>
<modules>
<module>moonbox-tools</module>
</modules>
<parent>
<groupId>com.flux.scev6</groupId>
<artifactId>datahub-utils-parentPom</artifactId>
<version>V10.0.0-P01-SNAPSHOT</version>
</parent>
</project>`
	childPOM := `<project>
<parent>
<groupId>com.flux.wmsv6</groupId>
<artifactId>flux-collaboration-storage-repeater-parent</artifactId>
<version>V10.0.0-P01</version>
</parent>
<artifactId>moonbox-tools</artifactId>
</project>`
	if err := os.WriteFile(filepath.Join(dir, "pom.xml"), []byte(rootPOM), 0o644); err != nil {
		t.Fatal(err)
	}
	childDir := filepath.Join(dir, "moonbox-tools")
	if err := os.MkdirAll(childDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(childDir, "pom.xml"), []byte(childPOM), 0o644); err != nil {
		t.Fatal(err)
	}

	tag := "V10.0.0-P02"
	pomVer := PomVersionForTag(tag)
	files, steps, err := CollectPomVersionUpdates(dir, tag)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 {
		t.Fatalf("files=%d steps=%v", len(files), steps)
	}
	rootOut := string(files[filepath.Join(dir, "pom.xml")])
	childOut := string(files[filepath.Join(childDir, "pom.xml")])
	if strings.Count(rootOut, "<version>"+pomVer+"</version>") < 2 {
		t.Fatalf("root pom versions not updated: %s", rootOut)
	}
	if !strings.Contains(childOut, "<version>"+pomVer+"</version>") {
		t.Fatalf("child parent version not updated: %s", childOut)
	}
}

func TestDiscoverPomPaths_skipsTarget(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "pom.xml"), []byte("<project/>"), 0o644); err != nil {
		t.Fatal(err)
	}
	targetDir := filepath.Join(dir, "target")
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(targetDir, "pom.xml"), []byte("<project/>"), 0o644); err != nil {
		t.Fatal(err)
	}
	paths, err := DiscoverPomPaths(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) != 1 {
		t.Fatalf("paths=%v", paths)
	}
}

func TestUpdatePomVersions_skipNoProjectVersion(t *testing.T) {
	const pom = `<project><modelVersion>4.0.0</modelVersion></project>`
	_, skip, err := updatePomVersions(pom, "V9.0.0-P01-01", "")
	if err != nil {
		t.Fatal(err)
	}
	if skip == "" {
		t.Fatal("expected skip")
	}
}
