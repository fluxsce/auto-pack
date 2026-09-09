package mavendeps

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEffectiveDependencies_inheritsParent(t *testing.T) {
	dir := t.TempDir()
	parentDir := filepath.Join(dir, "parent")
	childDir := filepath.Join(dir, "child")
	if err := os.MkdirAll(parentDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(childDir, 0o755); err != nil {
		t.Fatal(err)
	}
	parentPOM := `<project>
<groupId>com.flux.wmsv6</groupId>
<artifactId>storage-parent</artifactId>
<version>V10.0.0-P01</version>
<dependencies>
  <dependency>
    <groupId>com.flux.wmsv6</groupId>
    <artifactId>flux-collaboration-storage-distribute</artifactId>
    <version>V10.0.0-P01</version>
  </dependency>
</dependencies>
</project>`
	childPOM := `<project>
<parent>
  <groupId>com.flux.wmsv6</groupId>
  <artifactId>storage-parent</artifactId>
  <version>V10.0.0-P01</version>
  <relativePath>../parent/pom.xml</relativePath>
</parent>
<groupId>com.flux.wmsv6</groupId>
<artifactId>flux-collaboration-storage-common</artifactId>
<version>V10.0.0-P01</version>
<dependencies>
  <dependency>
    <groupId>com.flux.wmsv6</groupId>
    <artifactId>flux-collaboration-storage-netty</artifactId>
    <version>V10.0.0-P01</version>
  </dependency>
</dependencies>
</project>`
	if err := os.WriteFile(filepath.Join(parentDir, "pom.xml"), []byte(parentPOM), 0o644); err != nil {
		t.Fatal(err)
	}
	childPath := filepath.Join(childDir, "pom.xml")
	if err := os.WriteFile(childPath, []byte(childPOM), 0o644); err != nil {
		t.Fatal(err)
	}
	lookup := buildPOMLookup([]ScanEntry{
		{ProjectDir: parentDir, ProjectVersion: "V10.0.0-P01"},
		{ProjectDir: childDir, ProjectVersion: "V10.0.0-P01"},
	})
	deps, err := EffectiveDependencies(childPath, "", lookup, map[string]string{"project.version": "V10.0.0-P01"})
	if err != nil {
		t.Fatal(err)
	}
	byArtifact := map[string]bool{}
	for _, d := range deps {
		byArtifact[d.ArtifactID] = true
	}
	if !byArtifact["flux-collaboration-storage-distribute"] {
		t.Fatalf("missing inherited distribute dep: %+v", deps)
	}
	if !byArtifact["flux-collaboration-storage-netty"] {
		t.Fatalf("missing child netty dep: %+v", deps)
	}
}

func TestResolveLocalRepository_mavenHomeDefault(t *testing.T) {
	dir := t.TempDir()
	settings := filepath.Join(dir, "settings.xml")
	if err := os.WriteFile(settings, []byte("<settings></settings>"), 0o644); err != nil {
		t.Fatal(err)
	}
	mavenHome := filepath.Join(dir, "apache-maven")
	got := ResolveLocalRepository(settings, mavenHome)
	want := filepath.Join(mavenHome, "localRepository")
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestCollectEffectiveProperties_grandparentProperty(t *testing.T) {
	dir := t.TempDir()
	grandDir := filepath.Join(dir, "grand")
	parentDir := filepath.Join(dir, "parent")
	childDir := filepath.Join(dir, "child")
	for _, d := range []string{grandDir, parentDir, childDir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	grandPOM := `<project>
<groupId>com.flux.scev6</groupId>
<artifactId>datahub-utils-parentPom</artifactId>
<version>V10.0.0-P01</version>
<properties>
  <datahub.core.version>V10.0.0-P01</datahub.core.version>
</properties>
</project>`
	parentPOM := `<project>
<parent>
  <groupId>com.flux.scev6</groupId>
  <artifactId>datahub-utils-parentPom</artifactId>
  <version>V10.0.0-P01</version>
</parent>
<groupId>com.flux.wmsv6</groupId>
<artifactId>storage-parent</artifactId>
<version>${datahub.core.version}</version>
<dependencies>
  <dependency>
    <groupId>com.flux.wmsv6</groupId>
    <artifactId>flux-collaboration-storage-distribute</artifactId>
    <version>${datahub.core.version}</version>
  </dependency>
</dependencies>
</project>`
	childPOM := `<project>
<parent>
  <groupId>com.flux.wmsv6</groupId>
  <artifactId>storage-parent</artifactId>
  <version>${datahub.core.version}</version>
  <relativePath>../parent/pom.xml</relativePath>
</parent>
<groupId>com.flux.wmsv6</groupId>
<artifactId>flux-collaboration-storage-common</artifactId>
<version>${datahub.core.version}</version>
</project>`
	if err := os.WriteFile(filepath.Join(grandDir, "pom.xml"), []byte(grandPOM), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(parentDir, "pom.xml"), []byte(parentPOM), 0o644); err != nil {
		t.Fatal(err)
	}
	childPath := filepath.Join(childDir, "pom.xml")
	if err := os.WriteFile(childPath, []byte(childPOM), 0o644); err != nil {
		t.Fatal(err)
	}
	lookup := buildPOMLookup([]ScanEntry{
		{ProjectDir: grandDir, ProjectVersion: "V10.0.0-P01"},
		{ProjectDir: parentDir, ProjectVersion: "V10.0.0-P01"},
		{ProjectDir: childDir, ProjectVersion: "V10.0.0-P01"},
	})
	props := CollectEffectiveProperties(childPath, "", lookup, map[string]string{"project.version": "V10.0.0-P01"})
	if props["datahub.core.version"] != "V10.0.0-P01" {
		t.Fatalf("datahub.core.version=%q props=%+v", props["datahub.core.version"], props)
	}
	deps, err := EffectiveDependencies(childPath, "", lookup, props)
	if err != nil {
		t.Fatal(err)
	}
	var distribute *Coordinates
	for i := range deps {
		if deps[i].ArtifactID == "flux-collaboration-storage-distribute" {
			distribute = &deps[i]
			break
		}
	}
	if distribute == nil {
		t.Fatalf("missing distribute dep: %+v", deps)
	}
	if distribute.Version != "V10.0.0-P01" {
		t.Fatalf("distribute version=%q", distribute.Version)
	}
}

func TestResolveLocalRepository_ignoresPlaceholder(t *testing.T) {
	dir := t.TempDir()
	settings := filepath.Join(dir, "settings.xml")
	raw := "<settings><localRepository>/path/to/local/repo</localRepository></settings>"
	if err := os.WriteFile(settings, []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}
	mavenHome := filepath.Join(dir, "apache-maven")
	got := ResolveLocalRepository(settings, mavenHome)
	want := filepath.Join(mavenHome, "localRepository")
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}
