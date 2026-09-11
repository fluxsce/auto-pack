package order

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"auto-pack/server/internal/project"
)

func TestAnalyzeMavenWorkspaceOrder(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a")
	b := filepath.Join(dir, "b")
	_ = os.MkdirAll(a, 0o755)
	_ = os.MkdirAll(b, 0o755)
	if err := os.WriteFile(filepath.Join(a, "pom.xml"), []byte(`<project><groupId>g</groupId><artifactId>core</artifactId></project>`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(b, "pom.xml"), []byte(`<project><groupId>g</groupId><artifactId>app</artifactId><dependencies><dependency><groupId>g</groupId><artifactId>core</artifactId></dependency></dependencies></project>`), 0o644); err != nil {
		t.Fatal(err)
	}
	list := []project.Project{
		{RecID: "1", ProjectName: "core", ProjectType: "maven", IsEnabled: 1, LastJobStatus: "succeeded"},
		{RecID: "2", ProjectName: "app", ProjectType: "maven", IsEnabled: 1},
	}
	out := AnalyzeMavenWorkspace(list, func(p project.Project) string {
		if p.RecID == "1" {
			return a
		}
		return b
	})
	if len(out.FullBuildOrder) != 2 || out.FullBuildOrder[0].ProjectRecID != "1" || out.FullBuildOrder[1].ProjectRecID != "2" {
		t.Fatalf("%+v", out.FullBuildOrder)
	}
	if len(out.InternalEdges) != 1 {
		t.Fatalf("edges %+v", out.InternalEdges)
	}
	if out.FullBuildOrder[0].LastJobStatus != "succeeded" || out.FullBuildOrder[0].IsEnabled != 1 {
		t.Fatalf("job fields %+v", out.FullBuildOrder[0])
	}
}

func TestAnalyzeMavenWorkspaceParentGroupAndDepMgmt(t *testing.T) {
	dir := t.TempDir()
	core := filepath.Join(dir, "core")
	app := filepath.Join(dir, "app")
	lib := filepath.Join(dir, "lib")
	_ = os.MkdirAll(core, 0o755)
	_ = os.MkdirAll(app, 0o755)
	_ = os.MkdirAll(lib, 0o755)
	if err := os.WriteFile(filepath.Join(core, "pom.xml"), []byte(`<project>
		<parent><groupId>g</groupId><artifactId>parent</artifactId><version>1</version></parent>
		<artifactId>core</artifactId>
	</project>`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(app, "pom.xml"), []byte(`<project>
		<groupId>g</groupId><artifactId>app</artifactId>
		<dependencyManagement><dependencies>
			<dependency><groupId>g</groupId><artifactId>core</artifactId><version>1</version></dependency>
		</dependencies></dependencyManagement>
		<dependencies>
			<dependency><groupId>${project.groupId}</groupId><artifactId>lib</artifactId></dependency>
		</dependencies>
	</project>`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(lib, "pom.xml"), []byte(`<project><groupId>g</groupId><artifactId>lib</artifactId></project>`), 0o644); err != nil {
		t.Fatal(err)
	}
	list := []project.Project{
		{RecID: "1", ProjectName: "core", ProjectType: "maven"},
		{RecID: "2", ProjectName: "app", ProjectType: "maven"},
		{RecID: "3", ProjectName: "lib", ProjectType: "maven"},
	}
	out := AnalyzeMavenWorkspace(list, func(p project.Project) string {
		switch p.RecID {
		case "1":
			return core
		case "2":
			return app
		default:
			return lib
		}
	})
	if len(out.FullBuildOrder) != 3 {
		t.Fatalf("order %+v", out.FullBuildOrder)
	}
	if out.FullBuildOrder[0].Coordinates != "g:core" && out.FullBuildOrder[1].Coordinates != "g:core" && out.FullBuildOrder[2].Coordinates != "g:core" {
		t.Fatalf("parent group not inherited: %+v", out.FullBuildOrder)
	}
	if len(out.InternalEdges) != 1 || out.InternalEdges[0].ProviderRecID != "3" || out.InternalEdges[0].ConsumerRecID != "2" {
		t.Fatalf("want app→lib only, got %+v", out.InternalEdges)
	}
	idx := map[string]int{}
	for i, m := range out.FullBuildOrder {
		idx[m.ProjectRecID] = i
	}
	if idx["3"] > idx["2"] {
		t.Fatalf("lib should precede app: %+v", out.FullBuildOrder)
	}
}

func TestAnalyzeMavenWorkspaceUnparsedAppended(t *testing.T) {
	dir := t.TempDir()
	ok := filepath.Join(dir, "ok")
	miss := filepath.Join(dir, "miss")
	_ = os.MkdirAll(ok, 0o755)
	_ = os.MkdirAll(miss, 0o755)
	if err := os.WriteFile(filepath.Join(ok, "pom.xml"), []byte(`<project><groupId>g</groupId><artifactId>ok</artifactId></project>`), 0o644); err != nil {
		t.Fatal(err)
	}
	list := []project.Project{
		{RecID: "2", ProjectName: "zeta", ProjectType: "maven"},
		{RecID: "1", ProjectName: "ok", ProjectType: "maven"},
	}
	out := AnalyzeMavenWorkspace(list, func(p project.Project) string {
		if p.RecID == "1" {
			return ok
		}
		return miss
	})
	if len(out.FullBuildOrder) != 2 || out.FullBuildOrder[0].ProjectRecID != "1" || out.FullBuildOrder[1].ProjectRecID != "2" {
		t.Fatalf("%+v", out.FullBuildOrder)
	}
	if len(out.Warnings) != 1 || !strings.Contains(out.Warnings[0], "zeta") {
		t.Fatalf("warnings %+v", out.Warnings)
	}
}

func TestAnalyzeWorkspaceNpmOrder(t *testing.T) {
	dir := t.TempDir()
	core := filepath.Join(dir, "core")
	app := filepath.Join(dir, "app")
	_ = os.MkdirAll(core, 0o755)
	_ = os.MkdirAll(app, 0o755)
	if err := os.WriteFile(filepath.Join(core, "package.json"), []byte(`{"name":"@ws/core","version":"1.0.0"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(app, "package.json"), []byte(`{"name":"@ws/app","dependencies":{"@ws/core":"workspace:*"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	list := []project.Project{
		{RecID: "2", ProjectName: "app", ProjectType: "npm"},
		{RecID: "1", ProjectName: "core", ProjectType: "vue"},
	}
	out := AnalyzeMavenWorkspace(list, func(p project.Project) string {
		if p.RecID == "1" {
			return core
		}
		return app
	})
	if len(out.FullBuildOrder) != 2 || out.FullBuildOrder[0].ProjectRecID != "1" || out.FullBuildOrder[1].ProjectRecID != "2" {
		t.Fatalf("order %+v", out.FullBuildOrder)
	}
	if len(out.InternalEdges) != 1 || out.InternalEdges[0].ProviderRecID != "1" {
		t.Fatalf("edges %+v", out.InternalEdges)
	}
}

func TestAnalyzeMavenWorkspaceCycleSortedByName(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a")
	b := filepath.Join(dir, "b")
	_ = os.MkdirAll(a, 0o755)
	_ = os.MkdirAll(b, 0o755)
	if err := os.WriteFile(filepath.Join(a, "pom.xml"), []byte(`<project><groupId>g</groupId><artifactId>beta</artifactId><dependencies><dependency><groupId>g</groupId><artifactId>alpha</artifactId></dependency></dependencies></project>`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(b, "pom.xml"), []byte(`<project><groupId>g</groupId><artifactId>alpha</artifactId><dependencies><dependency><groupId>g</groupId><artifactId>beta</artifactId></dependency></dependencies></project>`), 0o644); err != nil {
		t.Fatal(err)
	}
	list := []project.Project{
		{RecID: "2", ProjectName: "beta", ProjectType: "maven"},
		{RecID: "1", ProjectName: "alpha", ProjectType: "maven"},
	}
	out := AnalyzeMavenWorkspace(list, func(p project.Project) string {
		if p.ProjectName == "beta" {
			return a
		}
		return b
	})
	if len(out.CyclicGroups) != 1 {
		t.Fatalf("cycle %+v", out)
	}
	if out.FullBuildOrder[0].ProjectName != "alpha" || out.FullBuildOrder[1].ProjectName != "beta" {
		t.Fatalf("want name order, got %+v", out.FullBuildOrder)
	}
}

func TestAnalyzeMavenWorkspaceUnmatchedSnapshot(t *testing.T) {
	dir := t.TempDir()
	app := filepath.Join(dir, "app")
	_ = os.MkdirAll(app, 0o755)
	if err := os.WriteFile(filepath.Join(app, "pom.xml"), []byte(`<project>
		<groupId>com.flux.wmsv6</groupId><artifactId>app</artifactId>
		<dependencies>
			<dependency>
				<groupId>com.flux.wmsv6</groupId>
				<artifactId>flux-collaboration-storage</artifactId>
				<version>10.0.0-P05-01-SNAPSHOT</version>
			</dependency>
			<dependency>
				<groupId>org.springframework</groupId>
				<artifactId>spring-core</artifactId>
				<version>6.1.0</version>
			</dependency>
		</dependencies>
	</project>`), 0o644); err != nil {
		t.Fatal(err)
	}
	list := []project.Project{{RecID: "1", ProjectName: "app", ProjectType: "maven"}}
	out := AnalyzeMavenWorkspace(list, func(project.Project) string { return app })
	if len(out.UnmatchedDeps) != 1 {
		t.Fatalf("unmatched %+v", out.UnmatchedDeps)
	}
	if out.UnmatchedDeps[0].Coordinates != "com.flux.wmsv6:flux-collaboration-storage:10.0.0-P05-01-SNAPSHOT" {
		t.Fatalf("coord %s", out.UnmatchedDeps[0].Coordinates)
	}
}
