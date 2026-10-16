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

func TestAnalyzeMavenWorkspaceImportBOMBuiltFirst(t *testing.T) {
	dir := t.TempDir()
	bom := filepath.Join(dir, "bom")
	app := filepath.Join(dir, "app")
	_ = os.MkdirAll(bom, 0o755)
	_ = os.MkdirAll(app, 0o755)
	if err := os.WriteFile(filepath.Join(bom, "pom.xml"), []byte(`<project>
		<groupId>g</groupId><artifactId>platform-bom</artifactId><version>1.0-SNAPSHOT</version>
		<packaging>pom</packaging>
	</project>`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(app, "pom.xml"), []byte(`<project>
		<groupId>g</groupId><artifactId>app</artifactId><version>1.0-SNAPSHOT</version>
		<dependencyManagement>
			<dependencies>
				<dependency>
					<groupId>g</groupId>
					<artifactId>platform-bom</artifactId>
					<version>1.0-SNAPSHOT</version>
					<type>pom</type>
					<scope>import</scope>
				</dependency>
			</dependencies>
		</dependencyManagement>
	</project>`), 0o644); err != nil {
		t.Fatal(err)
	}
	list := []project.Project{
		{RecID: "app", ProjectName: "app", ProjectType: "maven", IsEnabled: 1},
		{RecID: "bom", ProjectName: "bom", ProjectType: "maven", IsEnabled: 1},
	}
	out := AnalyzeMavenWorkspace(list, func(p project.Project) string {
		if p.RecID == "bom" {
			return bom
		}
		return app
	})
	if len(out.InternalEdges) != 1 || out.InternalEdges[0].ProviderRecID != "bom" || out.InternalEdges[0].ConsumerRecID != "app" {
		t.Fatalf("want app→bom import edge, got %+v", out.InternalEdges)
	}
	idx := map[string]int{}
	for i, m := range out.FullBuildOrder {
		idx[m.ProjectRecID] = i
	}
	if idx["bom"] > idx["app"] {
		t.Fatalf("bom should precede app: %+v", out.FullBuildOrder)
	}
}

func TestAnalyzeMavenWorkspacePropertyVersionEdge(t *testing.T) {
	dir := t.TempDir()
	lib := filepath.Join(dir, "lib")
	app := filepath.Join(dir, "app")
	_ = os.MkdirAll(lib, 0o755)
	_ = os.MkdirAll(app, 0o755)
	if err := os.WriteFile(filepath.Join(lib, "pom.xml"), []byte(`<project>
		<groupId>g</groupId><artifactId>lib</artifactId><version>${revision}</version>
		<properties><revision>1.0-SNAPSHOT</revision></properties>
	</project>`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(app, "pom.xml"), []byte(`<project>
		<groupId>g</groupId><artifactId>app</artifactId>
		<properties><lib.version>1.0-SNAPSHOT</lib.version></properties>
		<dependencies>
			<dependency>
				<groupId>g</groupId>
				<artifactId>lib</artifactId>
				<version>${lib.version}</version>
			</dependency>
		</dependencies>
	</project>`), 0o644); err != nil {
		t.Fatal(err)
	}
	list := []project.Project{
		{RecID: "app", ProjectName: "app", ProjectType: "maven", IsEnabled: 1},
		{RecID: "lib", ProjectName: "lib", ProjectType: "maven", IsEnabled: 1},
	}
	out := AnalyzeMavenWorkspace(list, func(p project.Project) string {
		if p.RecID == "lib" {
			return lib
		}
		return app
	})
	if len(out.InternalEdges) != 1 || out.InternalEdges[0].ProviderRecID != "lib" {
		t.Fatalf("want property-resolved edge, got %+v unmatched=%+v", out.InternalEdges, out.UnmatchedDeps)
	}
	idx := map[string]int{}
	for i, m := range out.FullBuildOrder {
		idx[m.ProjectRecID] = i
	}
	if idx["lib"] > idx["app"] {
		t.Fatalf("lib should precede app: %+v", out.FullBuildOrder)
	}
}

func TestAnalyzeMavenWorkspaceDisabledUpstreamWarning(t *testing.T) {
	dir := t.TempDir()
	lib := filepath.Join(dir, "lib")
	app := filepath.Join(dir, "app")
	_ = os.MkdirAll(lib, 0o755)
	_ = os.MkdirAll(app, 0o755)
	if err := os.WriteFile(filepath.Join(lib, "pom.xml"), []byte(`<project><groupId>g</groupId><artifactId>lib</artifactId></project>`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(app, "pom.xml"), []byte(`<project>
		<groupId>g</groupId><artifactId>app</artifactId>
		<dependencies><dependency><groupId>g</groupId><artifactId>lib</artifactId><version>1</version></dependency></dependencies>
	</project>`), 0o644); err != nil {
		t.Fatal(err)
	}
	list := []project.Project{
		{RecID: "lib", ProjectName: "lib", ProjectType: "maven", IsEnabled: 0},
		{RecID: "app", ProjectName: "app", ProjectType: "maven", IsEnabled: 1},
	}
	out := AnalyzeMavenWorkspace(list, func(p project.Project) string {
		if p.RecID == "lib" {
			return lib
		}
		return app
	})
	found := false
	for _, w := range out.Warnings {
		if strings.Contains(w, "已停用") && strings.Contains(w, "lib") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("warnings %+v", out.Warnings)
	}
}

func TestAnalyzeMavenWorkspaceParentBuiltFirst(t *testing.T) {
	dir := t.TempDir()
	parent := filepath.Join(dir, "parent")
	child := filepath.Join(dir, "child")
	_ = os.MkdirAll(parent, 0o755)
	_ = os.MkdirAll(child, 0o755)
	if err := os.WriteFile(filepath.Join(parent, "pom.xml"), []byte(`<project>
		<groupId>g</groupId><artifactId>parent-pom</artifactId><version>1.0-SNAPSHOT</version>
		<packaging>pom</packaging>
	</project>`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(child, "pom.xml"), []byte(`<project>
		<parent><groupId>g</groupId><artifactId>parent-pom</artifactId><version>1.0-SNAPSHOT</version></parent>
		<artifactId>child</artifactId>
	</project>`), 0o644); err != nil {
		t.Fatal(err)
	}
	list := []project.Project{
		{RecID: "c", ProjectName: "child", ProjectType: "maven"},
		{RecID: "p", ProjectName: "parent", ProjectType: "maven"},
	}
	out := AnalyzeMavenWorkspace(list, func(p project.Project) string {
		if p.RecID == "p" {
			return parent
		}
		return child
	})
	if len(out.InternalEdges) != 1 || out.InternalEdges[0].ProviderRecID != "p" || out.InternalEdges[0].ConsumerRecID != "c" {
		t.Fatalf("want child→parent edge, got %+v", out.InternalEdges)
	}
	idx := map[string]int{}
	for i, m := range out.FullBuildOrder {
		idx[m.ProjectRecID] = i
	}
	if idx["p"] > idx["c"] {
		t.Fatalf("parent should precede child: %+v", out.FullBuildOrder)
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

func TestAnalyzeMavenWorkspaceNestedSpringBootModule(t *testing.T) {
	dir := t.TempDir()
	parent := filepath.Join(dir, "repeater")
	nested := filepath.Join(parent, "fluxux-collaboration-storage-repeater")
	app := filepath.Join(dir, "app")
	_ = os.MkdirAll(nested, 0o755)
	_ = os.MkdirAll(app, 0o755)
	if err := os.WriteFile(filepath.Join(parent, "pom.xml"), []byte(`<project>
		<groupId>com.fluxux.wmsv6</groupId>
		<artifactId>repeater-parent</artifactId>
		<packaging>pom</packaging>
		<modules>
			<module>fluxux-collaboration-storage-repeater</module>
			<module>moonbox-core</module>
		</modules>
	</project>`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nested, "pom.xml"), []byte(`<project>
		<parent><groupId>com.fluxux.wmsv6</groupId><artifactId>repeater-parent</artifactId><version>10.0.0-P05-SNAPSHOT</version></parent>
		<artifactId>fluxux-collaboration-storage-repeater</artifactId>
	</project>`), 0o644); err != nil {
		t.Fatal(err)
	}
	_ = os.MkdirAll(filepath.Join(parent, "moonbox-core"), 0o755)
	if err := os.WriteFile(filepath.Join(parent, "moonbox-core", "pom.xml"), []byte(`<project>
		<parent><groupId>com.fluxux.wmsv6</groupId><artifactId>repeater-parent</artifactId><version>10.0.0-P05-SNAPSHOT</version></parent>
		<artifactId>moonbox-core</artifactId>
	</project>`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(app, "pom.xml"), []byte(`<project>
		<groupId>com.fluxux.wmsv6</groupId><artifactId>storage-agent</artifactId>
		<dependencies>
			<dependency>
				<groupId>com.fluxux.wmsv6</groupId>
				<artifactId>fluxux-collaboration-storage-repeater</artifactId>
				<version>10.0.0-P05-SNAPSHOT</version>
			</dependency>
		</dependencies>
	</project>`), 0o644); err != nil {
		t.Fatal(err)
	}
	list := []project.Project{
		{RecID: "1", ProjectName: "fluxux-collaboration-storage-repeater", ProjectType: "maven", IsEnabled: 1},
		{RecID: "2", ProjectName: "storage-agent", ProjectType: "maven", IsEnabled: 1},
	}
	out := AnalyzeMavenWorkspace(list, func(p project.Project) string {
		if p.RecID == "1" {
			return parent
		}
		return app
	})
	if len(out.UnmatchedDeps) != 0 {
		t.Fatalf("nested module should match workspace project, unmatched %+v", out.UnmatchedDeps)
	}
	if len(out.InternalEdges) != 1 || out.InternalEdges[0].ProviderRecID != "1" || out.InternalEdges[0].ConsumerRecID != "2" {
		t.Fatalf("want agent→repeater, got %+v", out.InternalEdges)
	}
}

func TestAnalyzeMavenWorkspaceSelfNestedNotUnmatched(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, "repeater")
	nested := filepath.Join(root, "fluxux-collaboration-storage-repeater")
	_ = os.MkdirAll(nested, 0o755)
	if err := os.WriteFile(filepath.Join(root, "pom.xml"), []byte(`<project>
		<groupId>com.fluxux.wmsv6</groupId>
		<artifactId>repeater-parent</artifactId>
		<modules><module>fluxux-collaboration-storage-repeater</module></modules>
		<dependencies>
			<dependency>
				<groupId>com.fluxux.wmsv6</groupId>
				<artifactId>fluxux-collaboration-storage-repeater</artifactId>
				<version>10.0.0-P05-SNAPSHOT</version>
			</dependency>
		</dependencies>
	</project>`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nested, "pom.xml"), []byte(`<project>
		<parent><groupId>com.fluxux.wmsv6</groupId><artifactId>repeater-parent</artifactId><version>1</version></parent>
		<artifactId>fluxux-collaboration-storage-repeater</artifactId>
	</project>`), 0o644); err != nil {
		t.Fatal(err)
	}
	list := []project.Project{{RecID: "1", ProjectName: "repeater", ProjectType: "maven"}}
	out := AnalyzeMavenWorkspace(list, func(project.Project) string { return root })
	if len(out.UnmatchedDeps) != 0 {
		t.Fatalf("own nested module is not missing: %+v", out.UnmatchedDeps)
	}
	if len(out.InternalEdges) != 0 {
		t.Fatalf("self module should not be an edge: %+v", out.InternalEdges)
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
