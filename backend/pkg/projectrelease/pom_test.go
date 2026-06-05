package projectrelease

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUpdatePomVersions_datahub(t *testing.T) {
	pomPath := filepath.Join(
		"..", "..", "data", "workspaces", "V9.0.0-P01", "flux-collaboration-datahub01api", "pom.xml",
	)
	raw, err := os.ReadFile(pomPath)
	if err != nil {
		t.Skip("testdata not present:", err)
	}
	tag := "V9.0.0-P08"
	out, err := UpdatePomVersions(string(raw), tag)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "<version>"+tag+"</version>") {
		t.Fatal("expected project/parent version updated")
	}
	if !strings.Contains(out, "<datahub.core.version>"+tag+"</datahub.core.version>") {
		t.Fatal("expected datahub.core.version inserted")
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
	out, err := UpdatePomVersions(string(raw), tag)
	if err != nil {
		t.Fatal(err)
	}
	count := strings.Count(out, "<version>"+tag+"</version>")
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
	out, skip, err := updatePomVersions(pom, "V9.0.0-P01-01")
	if err != nil || skip != "" {
		t.Fatalf("err=%v skip=%q", err, skip)
	}
	if !strings.Contains(out, "<version>1.0.0</version>") {
		t.Fatal("other parent version should stay unchanged")
	}
	if strings.Count(out, "<version>V9.0.0-P01-01</version>") != 1 {
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
	out, skip, err := updatePomVersions(pom, "V9.0.0-P01-01")
	if err != nil || skip != "" {
		t.Fatalf("err=%v skip=%q", err, skip)
	}
	if !strings.Contains(out, "<artifactId>datahub-utils-parentPom</artifactId>") {
		t.Fatal("parent block missing")
	}
	if strings.Count(out, "<version>V9.0.0-P01-01</version>") != 2 {
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
	out, skip, err := updatePomVersions(pom, "V9.0.0-P01-01")
	if err != nil || skip != "" {
		t.Fatalf("err=%v skip=%q", err, skip)
	}
	if !strings.Contains(out, "<version>V9.0.0-P01-01</version>") {
		t.Fatalf("got %s", out)
	}
}

func TestUpdatePomVersions_skipNoProjectVersion(t *testing.T) {
	const pom = `<project><modelVersion>4.0.0</modelVersion></project>`
	_, skip, err := updatePomVersions(pom, "V9.0.0-P01-01")
	if err != nil {
		t.Fatal(err)
	}
	if skip == "" {
		t.Fatal("expected skip")
	}
}
