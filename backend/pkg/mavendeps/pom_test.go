package mavendeps

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParsePOM_doesNotReadCoordinatesFromDependencies(t *testing.T) {
	const pom = `<project>
<parent>
  <groupId>com.flux.wmsv6</groupId>
  <artifactId>datahub-parent</artifactId>
  <version>10.0.0-SNAPSHOT</version>
</parent>
<properties>
  <datahub.core.version>V10.0.0-P01</datahub.core.version>
</properties>
<dependencies>
  <dependency>
    <groupId>com.flux.wmsv6</groupId>
    <artifactId>flux-collaboration-storage</artifactId>
    <version>${datahub.core.version}</version>
  </dependency>
</dependencies>
</project>`
	_, artifactID, _ := parseProjectCoordinates(pom)
	if artifactID != "" {
		t.Fatalf("artifactId=%q want empty (must not read from dependencies)", artifactID)
	}
	g, _, _ := parseProjectCoordinates(pom)
	if g != "" {
		t.Fatalf("groupId=%q want empty without project coordinates", g)
	}
}

func TestResolveModuleCoordinates_keepsExplicitSnapshotVersion(t *testing.T) {
	dir := t.TempDir()
	repo := filepath.Join(dir, "repo")
	parentPomDir := filepath.Join(repo, "com", "flux", "scev6", "datahub-utils-parentPom", "V10.0.0-P01-SNAPSHOT")
	if err := os.MkdirAll(parentPomDir, 0o755); err != nil {
		t.Fatal(err)
	}
	parentPOM := `<project>
<groupId>com.flux.scev6</groupId>
<artifactId>datahub-utils-parentPom</artifactId>
<version>10.0.0-SNAPSHOT</version>
</project>`
	if err := os.WriteFile(filepath.Join(parentPomDir, "datahub-utils-parentPom-V10.0.0-P01-SNAPSHOT.pom"), []byte(parentPOM), 0o644); err != nil {
		t.Fatal(err)
	}
	moduleDir := filepath.Join(dir, "flux-collaboration-datahub01srv")
	if err := os.MkdirAll(moduleDir, 0o755); err != nil {
		t.Fatal(err)
	}
	modulePOM := `<project>
<groupId>com.flux.wmsv6</groupId>
<artifactId>flux-collaboration-datahub01srv</artifactId>
<version>V10.0.0-P01-SNAPSHOT</version>
<parent>
  <groupId>com.flux.scev6</groupId>
  <artifactId>datahub-utils-parentPom</artifactId>
  <version>V10.0.0-P01-SNAPSHOT</version>
</parent>
<properties>
  <datahub.core.version>V10.0.0-P01-SNAPSHOT</datahub.core.version>
</properties>
</project>`
	pomPath := filepath.Join(moduleDir, "pom.xml")
	if err := os.WriteFile(pomPath, []byte(modulePOM), 0o644); err != nil {
		t.Fatal(err)
	}
	coord, err := ResolveModuleCoordinates(pomPath, repo, pomLookup{}, "flux-collaboration-datahub01srv", "")
	if err != nil {
		t.Fatal(err)
	}
	if coord.Version != "V10.0.0-P01-SNAPSHOT" {
		t.Fatalf("coord version=%q want explicit module snapshot, not parent legacy version", coord.Version)
	}
}

func TestResolveModuleCoordinates_aggregatorUsesJarModule(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, "flux-collaboration-storage-repeater")
	child := filepath.Join(root, "flux-collaboration-storage-repeater")
	if err := os.MkdirAll(child, 0o755); err != nil {
		t.Fatal(err)
	}
	parentPOM := `<project>
<groupId>com.flux.wmsv6</groupId>
<artifactId>flux-collaboration-storage-repeater-parent</artifactId>
<version>V10.0.0-P01</version>
<packaging>pom</packaging>
<modules>
  <module>moonbox-tools</module>
  <module>flux-collaboration-storage-repeater</module>
</modules>
<parent>
  <groupId>com.flux.scev6</groupId>
  <artifactId>datahub-utils-parentPom</artifactId>
  <version>V10.0.0-P01</version>
</parent>
</project>`
	childPOM := `<project>
<parent>
  <groupId>com.flux.wmsv6</groupId>
  <artifactId>flux-collaboration-storage-repeater-parent</artifactId>
  <version>V10.0.0-P01</version>
</parent>
<artifactId>flux-collaboration-storage-repeater</artifactId>
<packaging>jar</packaging>
</project>`
	if err := os.WriteFile(filepath.Join(root, "pom.xml"), []byte(parentPOM), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(child, "pom.xml"), []byte(childPOM), 0o644); err != nil {
		t.Fatal(err)
	}
	coord, err := ResolveModuleCoordinates(filepath.Join(root, "pom.xml"), "", pomLookup{}, "flux-collaboration-storage-repeater", "V10.0.0-P01")
	if err != nil {
		t.Fatal(err)
	}
	if coord.ArtifactID != "flux-collaboration-storage-repeater" {
		t.Fatalf("coord=%+v want jar module artifact", coord)
	}
}

func TestResolveModuleCoordinates_groupIdBeforeParent(t *testing.T) {
	dir := t.TempDir()
	pomPath := filepath.Join(dir, "pom.xml")
	const pom = `<project xmlns="http://maven.apache.org/POM/4.0.0">
    <modelVersion>4.0.0</modelVersion>
    <groupId>com.flux.wmsv6</groupId>
    <artifactId>flux-collaboration-utils-external</artifactId>
    <version>V10.0.0-P01</version>
    <packaging>jar</packaging>
    <parent>
        <groupId>com.flux.scev6</groupId>
        <artifactId>datahub-utils-parentPom</artifactId>
        <version>V10.0.0-P01</version>
    </parent>
    <distributionManagement>
        <repository>
            <id>wmsv5-releases</id>
            <name>flux wmsv5 Release Repository</name>
        </repository>
    </distributionManagement>
    <build>
        <plugins>
            <plugin>
                <groupId>org.apache.maven.plugins</groupId>
                <artifactId>maven-compiler-plugin</artifactId>
            </plugin>
        </plugins>
    </build>
</project>`
	if err := os.WriteFile(pomPath, []byte(pom), 0o644); err != nil {
		t.Fatal(err)
	}
	coord, err := ResolveModuleCoordinates(pomPath, "", pomLookup{}, "flux-collaboration-utils-external", "V10.0.0-P01")
	if err != nil {
		t.Fatal(err)
	}
	if coord.GroupID != "com.flux.wmsv6" || coord.ArtifactID != "flux-collaboration-utils-external" {
		t.Fatalf("coord=%+v want com.flux.wmsv6:flux-collaboration-utils-external", coord)
	}
}

func TestResolveModuleCoordinates_ignoresBuildPlugins(t *testing.T) {
	dir := t.TempDir()
	pomPath := filepath.Join(dir, "pom.xml")
	const pom = `<project>
<parent>
  <groupId>com.flux.scev6</groupId>
  <artifactId>datahub-utils-parentPom</artifactId>
  <version>V10.0.0-P01</version>
</parent>
<build>
  <plugins>
    <plugin>
      <groupId>org.apache.maven.plugins</groupId>
      <artifactId>maven-compiler-plugin</artifactId>
    </plugin>
  </plugins>
</build>
<groupId>com.flux.wmsv6</groupId>
<artifactId>flux-collaboration-utils-external</artifactId>
<version>V10.0.0-P01</version>
</project>`
	if err := os.WriteFile(pomPath, []byte(pom), 0o644); err != nil {
		t.Fatal(err)
	}
	coord, err := ResolveModuleCoordinates(pomPath, "", pomLookup{}, "flux-collaboration-utils-external", "V10.0.0-P01")
	if err != nil {
		t.Fatal(err)
	}
	if coord.GroupID != "com.flux.wmsv6" || coord.ArtifactID != "flux-collaboration-utils-external" {
		t.Fatalf("coord=%+v", coord)
	}
}

func TestResolveModuleCoordinates_groupIdInActiveProfile(t *testing.T) {
	dir := t.TempDir()
	pomPath := filepath.Join(dir, "pom.xml")
	const pom = `<project>
<parent>
  <groupId>com.flux.scev6</groupId>
  <artifactId>datahub-utils-parentPom</artifactId>
  <version>V10.0.0-P01</version>
</parent>
<artifactId>flux-collaboration-utils-external</artifactId>
<version>V10.0.0-P01</version>
<profiles>
  <profile>
    <id>default</id>
    <activation><activeByDefault>true</activeByDefault></activation>
    <groupId>com.flux.wmsv6</groupId>
  </profile>
</profiles>
</project>`
	if err := os.WriteFile(pomPath, []byte(pom), 0o644); err != nil {
		t.Fatal(err)
	}
	coord, err := ResolveModuleCoordinates(pomPath, "", pomLookup{}, "flux-collaboration-utils-external", "V10.0.0-P01")
	if err != nil {
		t.Fatal(err)
	}
	if coord.GroupID != "com.flux.wmsv6" || coord.ArtifactID != "flux-collaboration-utils-external" {
		t.Fatalf("coord=%+v", coord)
	}
}

func TestResolveModuleCoordinates_groupIdProperty(t *testing.T) {
	dir := t.TempDir()
	pomPath := filepath.Join(dir, "pom.xml")
	const pom = `<project>
<parent>
  <groupId>com.flux.scev6</groupId>
  <artifactId>datahub-utils-parentPom</artifactId>
  <version>V10.0.0-P01</version>
</parent>
<properties>
  <wms.groupId>com.flux.wmsv6</wms.groupId>
</properties>
<groupId>${wms.groupId}</groupId>
<artifactId>flux-collaboration-utils-external</artifactId>
<version>V10.0.0-P01</version>
</project>`
	if err := os.WriteFile(pomPath, []byte(pom), 0o644); err != nil {
		t.Fatal(err)
	}
	coord, err := ResolveModuleCoordinates(pomPath, "", pomLookup{}, "flux-collaboration-utils-external", "V10.0.0-P01")
	if err != nil {
		t.Fatal(err)
	}
	if coord.GroupID != "com.flux.wmsv6" || coord.ArtifactID != "flux-collaboration-utils-external" {
		t.Fatalf("coord=%+v", coord)
	}
}

func TestResolveModuleCoordinates_groupIdAfterDependencies(t *testing.T) {
	dir := t.TempDir()
	pomPath := filepath.Join(dir, "pom.xml")
	const pom = `<project>
<parent>
  <groupId>com.flux.scev6</groupId>
  <artifactId>datahub-utils-parentPom</artifactId>
  <version>V10.0.0-P01</version>
</parent>
<dependencies>
  <dependency>
    <groupId>com.flux.wmsv6</groupId>
    <artifactId>flux-collaboration-storage</artifactId>
    <version>V10.0.0-P01</version>
  </dependency>
</dependencies>
<groupId>com.flux.wmsv6</groupId>
<artifactId>flux-collaboration-utils-external</artifactId>
<version>V10.0.0-P01</version>
</project>`
	if err := os.WriteFile(pomPath, []byte(pom), 0o644); err != nil {
		t.Fatal(err)
	}
	coord, err := ResolveModuleCoordinates(pomPath, "", pomLookup{}, "flux-collaboration-utils-external", "V10.0.0-P01")
	if err != nil {
		t.Fatal(err)
	}
	if coord.GroupID != "com.flux.wmsv6" || coord.ArtifactID != "flux-collaboration-utils-external" {
		t.Fatalf("coord=%+v", coord)
	}
}

func TestScanModules_parentOnlyUsesProjectNameAsArtifact(t *testing.T) {
	dir := t.TempDir()
	projectDir := filepath.Join(dir, "flux-collaboration-adapterexpress")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatal(err)
	}
	const pom = `<project>
<parent>
  <groupId>com.flux.wmsv6</groupId>
  <artifactId>datahub-parent</artifactId>
  <version>10.0.0-SNAPSHOT</version>
</parent>
<properties>
  <datahub.core.version>V10.0.0-P01</datahub.core.version>
</properties>
<dependencies>
  <dependency>
    <groupId>com.flux.wmsv6</groupId>
    <artifactId>flux-collaboration-storage</artifactId>
    <version>${datahub.core.version}</version>
  </dependency>
</dependencies>
</project>`
	if err := os.WriteFile(filepath.Join(projectDir, "pom.xml"), []byte(pom), 0o644); err != nil {
		t.Fatal(err)
	}
	storageDir := filepath.Join(dir, "flux-collaboration-storage")
	if err := os.MkdirAll(storageDir, 0o755); err != nil {
		t.Fatal(err)
	}
	storagePOM := `<project>
<groupId>com.flux.wmsv6</groupId>
<artifactId>flux-collaboration-storage</artifactId>
<version>V10.0.0-P01</version>
</project>`
	if err := os.WriteFile(filepath.Join(storageDir, "pom.xml"), []byte(storagePOM), 0o644); err != nil {
		t.Fatal(err)
	}
	entries := []ScanEntry{
		{ProjectRecID: "adapter", ProjectName: "flux-collaboration-adapterexpress", ProjectVersion: "V10.0.0-P01", ProjectDir: projectDir},
		{ProjectRecID: "storage", ProjectName: "flux-collaboration-storage", ProjectVersion: "V10.0.0-P01", ProjectDir: storageDir},
	}
	ws, _, err := ScanModules(entries, "")
	if err != nil {
		t.Fatal(err)
	}
	storage := ws.byRecID["storage"]
	adapter := ws.byRecID["adapter"]
	if storage == nil || adapter == nil {
		t.Fatal("missing modules")
	}
	if storage.Coordinates.ArtifactID != "flux-collaboration-storage" || storage.Coordinates.Version != "V10.0.0-P01" {
		t.Fatalf("storage coords=%+v", storage.Coordinates)
	}
	if adapter.Coordinates.ArtifactID != "flux-collaboration-adapterexpress" {
		t.Fatalf("adapter artifactId=%q", adapter.Coordinates.ArtifactID)
	}
	if got := ws.byKey["com.flux.wmsv6:flux-collaboration-storage"]; got == nil || got.ProjectRecID != "storage" {
		t.Fatalf("byKey storage mapping=%v", got)
	}
	res, err := ws.AnalyzeTarget("adapter")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.BuildOrder) != 1 || res.BuildOrder[0].ProjectRecID != "storage" {
		t.Fatalf("build order=%+v", res.BuildOrder)
	}
}

func TestParsePOM_dependencies(t *testing.T) {
	const pom = `<project>
<groupId>com.flux.wmsv6</groupId>
<artifactId>downstream</artifactId>
<version>${datahub.core.version}</version>
<properties>
  <datahub.core.version>V9.0.0-P01-01</datahub.core.version>
</properties>
<dependencies>
  <dependency>
    <groupId>com.flux.wmsv6</groupId>
    <artifactId>upstream-lib</artifactId>
    <version>${datahub.core.version}</version>
  </dependency>
  <dependency>
    <groupId>junit</groupId>
    <artifactId>junit</artifactId>
    <version>4.13</version>
    <scope>test</scope>
  </dependency>
</dependencies>
</project>`
	p, err := ParsePOM(pom)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Dependencies) != 1 {
		t.Fatalf("deps len=%d want 1", len(p.Dependencies))
	}
	ver := p.ResolveVersion(p.Dependencies[0].Version, nil)
	if ver != "V9.0.0-P01-01" {
		t.Fatalf("resolved=%q", ver)
	}
}

func TestWorkspace_buildOrder(t *testing.T) {
	modules := []Module{
		{
			ProjectRecID: "up",
			ProjectName:  "upstream-lib",
			Coordinates:  Coordinates{GroupID: "com.flux", ArtifactID: "upstream-lib", Version: "1.0"},
		},
		{
			ProjectRecID: "down",
			ProjectName:  "downstream",
			Dependencies: []Coordinates{
				{GroupID: "com.flux", ArtifactID: "upstream-lib", Version: "1.0"},
			},
			Coordinates: Coordinates{GroupID: "com.flux", ArtifactID: "downstream", Version: "1.0"},
		},
	}
	ws := NewWorkspace(modules, "")
	res, err := ws.AnalyzeTarget("down")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.BuildOrder) != 1 || res.BuildOrder[0].ProjectRecID != "up" {
		t.Fatalf("order=%+v", res.BuildOrder)
	}
}

func TestTopoSort_duplicateDependencyEdges(t *testing.T) {
	modules := []Module{
		{
			ProjectRecID: "a",
			ProjectName:  "lib-a",
			Coordinates:  Coordinates{GroupID: "com.flux", ArtifactID: "lib-a", Version: "1.0"},
		},
		{
			ProjectRecID: "b",
			ProjectName:  "lib-b",
			Dependencies: []Coordinates{
				{GroupID: "com.flux", ArtifactID: "lib-a", Version: "1.0"},
				{GroupID: "com.flux", ArtifactID: "lib-a", Version: "1.0"},
			},
			Coordinates: Coordinates{GroupID: "com.flux", ArtifactID: "lib-b", Version: "1.0"},
		},
	}
	ws := NewWorkspace(modules, "")
	needed := map[string]bool{"a": true, "b": true}
	order, err := ws.topoSort(needed, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(order) != 2 || order[0].ProjectRecID != "a" {
		t.Fatalf("order=%v", order)
	}
}

func TestTopoSort_selfDependencyIgnored(t *testing.T) {
	modules := []Module{
		{
			ProjectRecID: "a",
			ProjectName:  "lib-a",
			Dependencies: []Coordinates{
				{GroupID: "com.flux", ArtifactID: "lib-a", Version: "1.0"},
			},
			Coordinates: Coordinates{GroupID: "com.flux", ArtifactID: "lib-a", Version: "1.0"},
		},
	}
	ws := NewWorkspace(modules, "")
	order, err := ws.topoSort(map[string]bool{"a": true}, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(order) != 1 {
		t.Fatalf("order=%v", order)
	}
}

func TestAnalyzeAll_cycleReturnsWarning(t *testing.T) {
	modules := []Module{
		{
			ProjectRecID: "a",
			ProjectName:  "mod-a",
			Dependencies: []Coordinates{
				{GroupID: "com.flux", ArtifactID: "mod-b", Version: "1.0"},
			},
			Coordinates: Coordinates{GroupID: "com.flux", ArtifactID: "mod-a", Version: "1.0"},
		},
		{
			ProjectRecID: "b",
			ProjectName:  "mod-b",
			Dependencies: []Coordinates{
				{GroupID: "com.flux", ArtifactID: "mod-a", Version: "1.0"},
			},
			Coordinates: Coordinates{GroupID: "com.flux", ArtifactID: "mod-b", Version: "1.0"},
		},
	}
	ws := NewWorkspace(modules, "")
	res, err := ws.AnalyzeAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Warnings) == 0 {
		t.Fatal("expected cycle warning")
	}
	if len(res.CyclicGroups) != 1 || len(res.CyclicGroups[0].ProjectRecIDs) != 2 {
		t.Fatalf("cyclic groups=%+v", res.CyclicGroups)
	}
	if len(res.FullBuildOrder) != 2 {
		t.Fatalf("full order=%d", len(res.FullBuildOrder))
	}
}

func TestCycleAwareOrder_prefersInstalledInSCC(t *testing.T) {
	dir := t.TempDir()
	repo := filepath.Join(dir, "repo")
	jarDir := filepath.Join(repo, "com", "flux", "wmsv6", "flux-collaboration-storage-flow", "1.0")
	if err := os.MkdirAll(jarDir, 0o755); err != nil {
		t.Fatal(err)
	}
	jar := filepath.Join(jarDir, "flux-collaboration-storage-flow-1.0.jar")
	if err := os.WriteFile(jar, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	modules := []Module{
		{
			ProjectRecID: "adapter",
			ProjectName:  "flux-collaboration-storage-adapter",
			Dependencies: []Coordinates{
				{GroupID: "com.flux.wmsv6", ArtifactID: "flux-collaboration-storage-flow", Version: "1.0"},
			},
			Coordinates: Coordinates{GroupID: "com.flux.wmsv6", ArtifactID: "flux-collaboration-storage-adapter", Version: "1.0"},
		},
		{
			ProjectRecID: "flow",
			ProjectName:  "flux-collaboration-storage-flow",
			Dependencies: []Coordinates{
				{GroupID: "com.flux.wmsv6", ArtifactID: "flux-collaboration-storage-adapter", Version: "1.0"},
			},
			Coordinates: Coordinates{GroupID: "com.flux.wmsv6", ArtifactID: "flux-collaboration-storage-flow", Version: "1.0"},
		},
		{
			ProjectRecID: "leaf",
			ProjectName:  "flux-collaboration-storage-leaf",
			Dependencies: []Coordinates{
				{GroupID: "com.flux.wmsv6", ArtifactID: "flux-collaboration-storage-adapter", Version: "1.0"},
			},
			Coordinates: Coordinates{GroupID: "com.flux.wmsv6", ArtifactID: "flux-collaboration-storage-leaf", Version: "1.0"},
		},
	}
	ws := NewWorkspace(modules, repo)
	res, err := ws.AnalyzeAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(res.FullBuildOrder) != 3 {
		t.Fatalf("full order=%v", res.FullBuildOrder)
	}
	if res.FullBuildOrder[0].ProjectRecID != "flow" {
		t.Fatalf("expected installed flow first, got %s", res.FullBuildOrder[0].ProjectName)
	}
	if res.FullBuildOrder[2].ProjectRecID != "leaf" {
		t.Fatalf("expected leaf last, got %s", res.FullBuildOrder[2].ProjectName)
	}
}

func TestWorkspace_analyzeAll(t *testing.T) {
	modules := []Module{
		{
			ProjectRecID: "a",
			ProjectName:  "lib-a",
			Coordinates:  Coordinates{GroupID: "com.flux", ArtifactID: "lib-a", Version: "1.0"},
		},
		{
			ProjectRecID: "b",
			ProjectName:  "lib-b",
			Dependencies: []Coordinates{
				{GroupID: "com.flux", ArtifactID: "lib-a", Version: "1.0"},
			},
			Coordinates: Coordinates{GroupID: "com.flux", ArtifactID: "lib-b", Version: "1.0"},
		},
		{
			ProjectRecID: "c",
			ProjectName:  "app-c",
			Dependencies: []Coordinates{
				{GroupID: "com.flux", ArtifactID: "lib-b", Version: "1.0"},
			},
			Coordinates: Coordinates{GroupID: "com.flux", ArtifactID: "app-c", Version: "1.0"},
		},
	}
	ws := NewWorkspace(modules, "")
	res, err := ws.AnalyzeAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(res.FullBuildOrder) != 3 {
		t.Fatalf("full order len=%d", len(res.FullBuildOrder))
	}
	if res.FullBuildOrder[0].ProjectRecID != "a" || res.FullBuildOrder[2].ProjectRecID != "c" {
		t.Fatalf("order=%v", res.FullBuildOrder)
	}
	if len(res.BuildOrder) != 3 {
		t.Fatalf("build order len=%d", len(res.BuildOrder))
	}
	if len(res.InternalEdges) != 2 {
		t.Fatalf("edges=%d", len(res.InternalEdges))
	}
}

func TestWorkspace_skipInstalled(t *testing.T) {
	dir := t.TempDir()
	repo := filepath.Join(dir, "repo")
	_ = os.MkdirAll(filepath.Join(repo, "com", "flux", "upstream-lib", "1.0"), 0o755)
	jar := filepath.Join(repo, "com", "flux", "upstream-lib", "1.0", "upstream-lib-1.0.jar")
	if err := os.WriteFile(jar, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	modules := []Module{
		{
			ProjectRecID: "up",
			Coordinates:  Coordinates{GroupID: "com.flux", ArtifactID: "upstream-lib", Version: "1.0"},
		},
		{
			ProjectRecID: "down",
			Dependencies: []Coordinates{
				{GroupID: "com.flux", ArtifactID: "upstream-lib", Version: "1.0"},
			},
			Coordinates: Coordinates{GroupID: "com.flux", ArtifactID: "downstream", Version: "1.0"},
		},
	}
	ws := NewWorkspace(modules, repo)
	res, err := ws.AnalyzeTarget("down")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.BuildOrder) != 0 || len(res.SkippedInstalled) != 1 {
		t.Fatalf("build=%d skipped=%d", len(res.BuildOrder), len(res.SkippedInstalled))
	}
}
