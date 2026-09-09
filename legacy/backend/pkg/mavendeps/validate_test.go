package mavendeps

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateTargetDependencies_workspaceUpstreamMissing(t *testing.T) {
	modules := []Module{
		{
			ProjectRecID: "storage",
			ProjectName:  "flux-collaboration-storage",
			Coordinates:  Coordinates{GroupID: "com.flux.wmsv6", ArtifactID: "flux-collaboration-storage", Version: "V10.0.0-P01"},
		},
		{
			ProjectRecID: "timer",
			ProjectName:  "flux-collaboration-timer",
			Dependencies: []Coordinates{
				{GroupID: "com.flux.wmsv6", ArtifactID: "flux-collaboration-storage", Version: "V10.0.0-P01"},
			},
			Coordinates: Coordinates{GroupID: "com.flux.wmsv6", ArtifactID: "flux-collaboration-timer", Version: "V10.0.0-P01"},
		},
	}
	dir := t.TempDir()
	repo := filepath.Join(dir, "repo")
	ws := NewWorkspace(modules, repo)
	unsatisfied := ws.ValidateTargetDependencies("timer")
	if len(unsatisfied) != 1 {
		t.Fatalf("unsatisfied=%+v", unsatisfied)
	}
	if unsatisfied[0].WorkspaceProject != "flux-collaboration-storage" {
		t.Fatalf("project=%q", unsatisfied[0].WorkspaceProject)
	}
}

func TestValidateTargetDependencies_workspaceUpstreamInstalled(t *testing.T) {
	dir := t.TempDir()
	repo := filepath.Join(dir, "repo")
	jarDir := filepath.Join(repo, "com", "flux", "wmsv6", "flux-collaboration-storage", "V10.0.0-P01")
	if err := os.MkdirAll(jarDir, 0o755); err != nil {
		t.Fatal(err)
	}
	jar := filepath.Join(jarDir, "flux-collaboration-storage-V10.0.0-P01.jar")
	if err := os.WriteFile(jar, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	modules := []Module{
		{
			ProjectRecID: "storage",
			ProjectName:  "flux-collaboration-storage",
			Coordinates:  Coordinates{GroupID: "com.flux.wmsv6", ArtifactID: "flux-collaboration-storage", Version: "V10.0.0-P01"},
		},
		{
			ProjectRecID: "timer",
			ProjectName:  "flux-collaboration-timer",
			Dependencies: []Coordinates{
				{GroupID: "com.flux.wmsv6", ArtifactID: "flux-collaboration-storage", Version: "V10.0.0-P01"},
			},
			Coordinates: Coordinates{GroupID: "com.flux.wmsv6", ArtifactID: "flux-collaboration-timer", Version: "V10.0.0-P01"},
		},
	}
	ws := NewWorkspace(modules, repo)
	if unsatisfied := ws.ValidateTargetDependencies("timer"); len(unsatisfied) != 0 {
		t.Fatalf("unsatisfied=%+v", unsatisfied)
	}
}

func TestValidateTargetDependencies_versionMismatch(t *testing.T) {
	modules := []Module{
		{
			ProjectRecID: "storage",
			ProjectName:  "flux-collaboration-storage",
			Coordinates:  Coordinates{GroupID: "com.flux.wmsv6", ArtifactID: "flux-collaboration-storage", Version: "V10.0.0-P01"},
		},
		{
			ProjectRecID: "timer",
			ProjectName:  "flux-collaboration-timer",
			Dependencies: []Coordinates{
				{GroupID: "com.flux.wmsv6", ArtifactID: "flux-collaboration-storage", Version: "V10.0.0-P02"},
			},
			Coordinates: Coordinates{GroupID: "com.flux.wmsv6", ArtifactID: "flux-collaboration-timer", Version: "V10.0.0-P01"},
		},
	}
	ws := NewWorkspace(modules, "")
	unsatisfied := ws.ValidateTargetDependencies("timer")
	if len(unsatisfied) != 1 {
		t.Fatalf("unsatisfied=%+v", unsatisfied)
	}
	if unsatisfied[0].WorkspaceProject != "flux-collaboration-storage" {
		t.Fatalf("project=%q", unsatisfied[0].WorkspaceProject)
	}
}

func TestValidateTargetDependencies_externalGroupSkipped(t *testing.T) {
	modules := []Module{
		{
			ProjectRecID: "timer",
			ProjectName:  "flux-collaboration-timer",
			Dependencies: []Coordinates{
				{GroupID: "org.springframework", ArtifactID: "spring-core", Version: "5.3.0"},
			},
			Coordinates: Coordinates{GroupID: "com.flux.wmsv6", ArtifactID: "flux-collaboration-timer", Version: "V10.0.0-P01"},
		},
	}
	ws := NewWorkspace(modules, t.TempDir())
	if unsatisfied := ws.ValidateTargetDependencies("timer"); len(unsatisfied) != 0 {
		t.Fatalf("unsatisfied=%+v", unsatisfied)
	}
}

func TestValidateTargetDependencies_sameGroupNotInWorkspace(t *testing.T) {
	modules := []Module{
		{
			ProjectRecID: "timer",
			ProjectName:  "flux-collaboration-timer",
			Dependencies: []Coordinates{
				{GroupID: "com.flux.wmsv6", ArtifactID: "flux-collaboration-storage", Version: "V10.0.0-P01"},
			},
			Coordinates: Coordinates{GroupID: "com.flux.wmsv6", ArtifactID: "flux-collaboration-timer", Version: "V10.0.0-P01"},
		},
	}
	ws := NewWorkspace(modules, t.TempDir())
	unsatisfied := ws.ValidateTargetDependencies("timer")
	if len(unsatisfied) != 1 {
		t.Fatalf("unsatisfied=%+v", unsatisfied)
	}
}

func TestAnalyzeTarget_staleProviderPomVersion(t *testing.T) {
	modules := []Module{
		{
			ProjectRecID: "storage",
			ProjectName:  "flux-collaboration-storage",
			Coordinates:  Coordinates{GroupID: "com.flux.wmsv6", ArtifactID: "flux-collaboration-storage", Version: "V10.0.0"},
		},
		{
			ProjectRecID: "timer",
			ProjectName:  "flux-collaboration-timer",
			Dependencies: []Coordinates{
				{GroupID: "com.flux.wmsv6", ArtifactID: "flux-collaboration-storage", Version: "V10.0.0-P01"},
			},
			Coordinates: Coordinates{GroupID: "com.flux.wmsv6", ArtifactID: "flux-collaboration-timer", Version: "V10.0.0-P01"},
		},
	}
	ws := NewWorkspace(modules, "")
	res, err := ws.AnalyzeTarget("timer")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.BuildOrder) != 1 || res.BuildOrder[0].ProjectRecID != "storage" {
		t.Fatalf("build order=%+v", res.BuildOrder)
	}
}

func TestWorkspaceArtifactSatisfied_aggregatorParentInstallPath(t *testing.T) {
	dir := t.TempDir()
	repo := filepath.Join(dir, "repo")
	jarDir := filepath.Join(repo, "com", "flux", "wmsv6", "flux-collaboration-storage-repeater", "V10.0.0-P01")
	if err := os.MkdirAll(jarDir, 0o755); err != nil {
		t.Fatal(err)
	}
	jar := filepath.Join(jarDir, "flux-collaboration-storage-repeater-V10.0.0-P01.jar")
	if err := os.WriteFile(jar, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	consumer := &Module{
		ProjectVersion: "V10.0.0-P01",
		Coordinates:    Coordinates{GroupID: "com.flux.wmsv6", ArtifactID: "flux-collaboration-ui2server", Version: "V10.0.0-P01"},
	}
	provider := &Module{
		ProjectName: "flux-collaboration-storage-repeater",
		Coordinates: Coordinates{
			GroupID: "com.flux.wmsv6", ArtifactID: "flux-collaboration-storage-repeater-parent", Version: "V10.0.0-P01",
		},
	}
	dep := Coordinates{
		GroupID: "com.flux.wmsv6", ArtifactID: "flux-collaboration-storage-repeater", Version: "V10.0.0-P01",
	}
	ws := NewWorkspace(nil, repo)
	if !ws.workspaceArtifactSatisfied(consumer, dep, provider) {
		t.Fatal("expected jar module path to satisfy aggregator workspace upstream")
	}
}

func TestWorkspaceArtifactSatisfied_fluxGroupIdInstallPath(t *testing.T) {
	dir := t.TempDir()
	repo := filepath.Join(dir, "repo")
	jarDir := filepath.Join(repo, "com", "flux", "wmsv6", "flux-collaboration-utils-external", "V10.0.0-P01")
	if err := os.MkdirAll(jarDir, 0o755); err != nil {
		t.Fatal(err)
	}
	jar := filepath.Join(jarDir, "flux-collaboration-utils-external-V10.0.0-P01.jar")
	if err := os.WriteFile(jar, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	consumer := &Module{
		ProjectVersion: "V10.0.0-P01",
		Coordinates:    Coordinates{GroupID: "com.flux.wmsv6", ArtifactID: "flux-datahub-cache", Version: "V10.0.0-P01"},
	}
	provider := &Module{
		ProjectName: "flux-collaboration-utils-external",
		Coordinates: Coordinates{
			GroupID: "com.flux.scev6", ArtifactID: "flux-collaboration-utils-external", Version: "V10.0.0-P01",
		},
	}
	dep := Coordinates{
		GroupID: "com.flux.scev6", ArtifactID: "flux-collaboration-utils-external", Version: "V10.0.0-P01",
	}
	ws := NewWorkspace(nil, repo)
	if !ws.workspaceArtifactSatisfied(consumer, dep, provider) {
		t.Fatal("expected jar under com.flux.wmsv6 to satisfy workspace upstream when scanned groupId is scev6")
	}
}

func TestFindWorkspaceModule_groupIdMismatch(t *testing.T) {
	modules := []Module{
		{
			ProjectRecID: "distribute",
			ProjectName:  "flux-collaboration-storage-distribute",
			Coordinates: Coordinates{
				GroupID: "com.flux.scev6", ArtifactID: "flux-collaboration-storage-distribute",
				Version: "V10.0.0-P01",
			},
		},
	}
	ws := NewWorkspace(modules, "")
	dep := Coordinates{GroupID: "com.flux.wmsv6", ArtifactID: "flux-collaboration-storage-distribute", Version: "V10.0.0-P01"}
	if m := ws.findWorkspaceModule(dep); m == nil || m.ProjectRecID != "distribute" {
		t.Fatalf("module=%v", m)
	}
}

func TestAnalyzeTarget_skipsWhenDependencyArtifactInstalled(t *testing.T) {
	dir := t.TempDir()
	repo := filepath.Join(dir, "repo")
	// flux-collaboration-services project publishes flux-collaboration-servers
	jarDir := filepath.Join(repo, "com", "flux", "wmsv6", "flux-collaboration-servers", "V10.0.0-P01")
	if err := os.MkdirAll(jarDir, 0o755); err != nil {
		t.Fatal(err)
	}
	jar := filepath.Join(jarDir, "flux-collaboration-servers-V10.0.0-P01.jar")
	if err := os.WriteFile(jar, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	ediDir := filepath.Join(repo, "com", "flux", "wmsv6", "flux-collaboration-conversion-edi", "V10.0.0-P01")
	if err := os.MkdirAll(ediDir, 0o755); err != nil {
		t.Fatal(err)
	}
	ediJar := filepath.Join(ediDir, "flux-collaboration-conversion-edi-V10.0.0-P01.jar")
	if err := os.WriteFile(ediJar, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	modules := []Module{
		{
			ProjectRecID:   "datahub",
			ProjectName:    "flux-collaboration-datahub",
			ProjectVersion: "V10.0.0-P01",
			Dependencies: []Coordinates{
				{GroupID: "com.flux.wmsv6", ArtifactID: "flux-collaboration-servers", Version: "V10.0.0-P01"},
				{GroupID: "com.flux.wmsv6", ArtifactID: "flux-collaboration-conversion-edi", Version: "V10.0.0-P01"},
			},
			Coordinates: Coordinates{
				GroupID: "com.flux.wmsv6", ArtifactID: "flux-collaboration-datahub", Version: "V10.0.0-P01",
			},
		},
		{
			ProjectRecID:   "services",
			ProjectName:    "flux-collaboration-services",
			ProjectVersion: "V10.0.0-P01",
			Coordinates: Coordinates{
				GroupID: "com.flux.wmsv6", ArtifactID: "flux-collaboration-servers", Version: "V10.0.0-P01",
			},
		},
		{
			ProjectRecID:   "edi",
			ProjectName:    "flux-collaboration-converver-edi",
			ProjectVersion: "V10.0.0-P01",
			Coordinates: Coordinates{
				GroupID: "com.flux.wmsv6", ArtifactID: "flux-collaboration-conversion-edi", Version: "V10.0.0-P01",
			},
		},
	}
	ws := NewWorkspace(modules, repo)
	res, err := ws.AnalyzeTarget("datahub")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.BuildOrder) != 0 {
		t.Fatalf("build order=%+v want skip when dependency GAV already installed", res.BuildOrder)
	}
	if len(res.SkippedInstalled) != 2 {
		t.Fatalf("skipped=%+v", res.SkippedInstalled)
	}
}

func TestAnalyzeTarget_storageUpstreamModules(t *testing.T) {
	upstreams := []string{
		"flux-collaboration-storage-distribute",
		"flux-collaboration-storage-common",
		"flux-collaboration-storage-netty",
	}
	modules := []Module{
		{
			ProjectRecID:   "storage",
			ProjectName:    "flux-collaboration-storage",
			ProjectVersion: "V10.0.0-P01",
			Dependencies: []Coordinates{
				{GroupID: "com.flux.wmsv6", ArtifactID: "flux-collaboration-storage-distribute", Version: "V10.0.0-P01"},
				{GroupID: "com.flux.wmsv6", ArtifactID: "flux-collaboration-storage-common", Version: "V10.0.0-P01"},
				{GroupID: "com.flux.wmsv6", ArtifactID: "flux-collaboration-storage-netty", Version: "V10.0.0-P01"},
			},
			Coordinates: Coordinates{
				GroupID: "com.flux.wmsv6", ArtifactID: "flux-collaboration-storage", Version: "V10.0.0-P01",
			},
		},
	}
	for i, name := range upstreams {
		modules = append(modules, Module{
			ProjectRecID:   fmt.Sprintf("up%d", i),
			ProjectName:    name,
			ProjectVersion: "V10.0.0-P01",
			Coordinates: Coordinates{
				GroupID: "com.flux.scev6", ArtifactID: name, Version: "V10.0.0-P01",
			},
		})
	}
	ws := NewWorkspace(modules, t.TempDir())
	res, err := ws.AnalyzeTarget("storage")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.BuildOrder) != 3 {
		t.Fatalf("build order=%+v", res.BuildOrder)
	}
}

func TestAnalyzeTarget_staleProviderSnapshotUsesConsumerRelease(t *testing.T) {
	dir := t.TempDir()
	repo := filepath.Join(dir, "repo")
	legacyDir := filepath.Join(repo, "com", "flux", "wmsv6", "flux-collaboration-servers", "10.0.0-SNAPSHOT")
	if err := os.MkdirAll(legacyDir, 0o755); err != nil {
		t.Fatal(err)
	}
	legacyJar := filepath.Join(legacyDir, "flux-collaboration-servers-10.0.0-SNAPSHOT.jar")
	if err := os.WriteFile(legacyJar, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	modules := []Module{
		{
			ProjectRecID:   "services",
			ProjectName:    "flux-collaboration-services",
			ProjectVersion: "develop",
			Coordinates: Coordinates{
				GroupID: "com.flux.wmsv6", ArtifactID: "flux-collaboration-servers",
				Version: "10.0.0-SNAPSHOT",
			},
		},
		{
			ProjectRecID:   "security",
			ProjectName:    "flux-collaboration-security",
			ProjectVersion: "develop",
			Dependencies: []Coordinates{
				{GroupID: "com.flux.wmsv6", ArtifactID: "flux-collaboration-servers", Version: "10.0.0-SNAPSHOT"},
			},
			Coordinates: Coordinates{
				GroupID: "com.flux.wmsv6", ArtifactID: "flux-collaboration-security",
				Version: "V10.0.0-P01-SNAPSHOT",
			},
		},
	}
	ws := NewWorkspace(modules, repo)
	coord := RequiredDependencyCoord(&modules[1], modules[1].Dependencies[0], &modules[0])
	if coord.Version != "V10.0.0-P01-SNAPSHOT" {
		t.Fatalf("coord version=%q want consumer release line", coord.Version)
	}
	res, err := ws.AnalyzeTarget("security")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.BuildOrder) != 1 || res.BuildOrder[0].ProjectRecID != "services" {
		t.Fatalf("build order=%+v want upstream install", res.BuildOrder)
	}
	unsatisfied := ws.ValidateTargetDependencies("security")
	if len(unsatisfied) != 1 {
		t.Fatalf("unsatisfied=%+v", unsatisfied)
	}
	if strings.Contains(unsatisfied[0].Reason, "版本未对齐") {
		t.Fatalf("expected missing install, got %q", unsatisfied[0].Reason)
	}
}

func TestAnalyzeTarget_storageSnapshotPomVersion(t *testing.T) {
	modules := []Module{
		{
			ProjectRecID:   "storage",
			ProjectName:    "flux-collaboration-storage",
			ProjectVersion: "V10.0.0-P01",
			Coordinates: Coordinates{
				GroupID: "com.flux.wmsv6", ArtifactID: "flux-collaboration-storage",
				Version: "10.0.0-SNAPSHOT",
			},
		},
		{
			ProjectRecID:   "adapterexpress",
			ProjectName:    "flux-collaboration-adapterexpress",
			ProjectVersion: "V10.0.0-P01",
			Dependencies: []Coordinates{
				{GroupID: "com.flux.wmsv6", ArtifactID: "flux-collaboration-storage", Version: "V10.0.0-P01"},
			},
			Coordinates: Coordinates{
				GroupID: "com.flux.wmsv6", ArtifactID: "flux-collaboration-adapterexpress",
				Version: "V10.0.0-P01",
			},
		},
	}
	ws := NewWorkspace(modules, t.TempDir())
	res, err := ws.AnalyzeTarget("adapterexpress")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.BuildOrder) != 1 || res.BuildOrder[0].ProjectRecID != "storage" {
		t.Fatalf("build order=%+v", res.BuildOrder)
	}
	coord := RequiredDependencyCoord(&modules[1], modules[1].Dependencies[0], &modules[0])
	if coord.Version != "V10.0.0-P01" {
		t.Fatalf("coord version=%q", coord.Version)
	}
	if unsatisfied := ws.ValidateTargetDependencies("adapterexpress"); len(unsatisfied) != 1 {
		t.Fatalf("unsatisfied=%+v", unsatisfied)
	}
}

func TestAnalyzeTarget_distributeUnresolvedProperty(t *testing.T) {
	modules := []Module{
		{
			ProjectRecID:   "distribute",
			ProjectName:    "flux-collaboration-storage-distribute",
			ProjectVersion: "V10.0.0",
			Coordinates: Coordinates{
				GroupID: "com.flux.wmsv6", ArtifactID: "flux-collaboration-storage-distribute",
				Version: "${datahub.core.version}",
			},
		},
		{
			ProjectRecID:   "common",
			ProjectName:    "flux-collaboration-storage-common",
			ProjectVersion: "V10.0.0",
			Dependencies: []Coordinates{
				{
					GroupID: "com.flux.wmsv6", ArtifactID: "flux-collaboration-storage-distribute",
					Version: "${datahub.core.version}",
				},
			},
			Coordinates: Coordinates{
				GroupID: "com.flux.wmsv6", ArtifactID: "flux-collaboration-storage-common",
				Version: "V10.0.0-P01",
			},
		},
	}
	ws := NewWorkspace(modules, t.TempDir())
	res, err := ws.AnalyzeTarget("common")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.BuildOrder) != 1 || res.BuildOrder[0].ProjectRecID != "distribute" {
		t.Fatalf("build order=%+v", res.BuildOrder)
	}
	if unsatisfied := ws.ValidateTargetDependencies("common"); len(unsatisfied) != 1 {
		t.Fatalf("unsatisfied=%+v", unsatisfied)
	}
}

func TestAnalyzeTarget_distributeByProjectName(t *testing.T) {
	modules := []Module{
		{
			ProjectRecID:   "distribute",
			ProjectName:    "flux-collaboration-storage-distribute",
			ProjectVersion: "V10.0.0-P01",
			Coordinates: Coordinates{
				GroupID: "com.flux.wmsv6", ArtifactID: "wrong-artifact-id",
				Version: "V10.0.0-P01",
			},
		},
		{
			ProjectRecID: "common",
			ProjectName:  "flux-collaboration-storage-common",
			Dependencies: []Coordinates{
				{GroupID: "com.flux.wmsv6", ArtifactID: "flux-collaboration-storage-distribute", Version: "V10.0.0-P01"},
			},
			Coordinates: Coordinates{
				GroupID: "com.flux.wmsv6", ArtifactID: "flux-collaboration-storage-common", Version: "V10.0.0-P01",
			},
		},
	}
	ws := NewWorkspace(modules, t.TempDir())
	res, err := ws.AnalyzeTarget("common")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.BuildOrder) != 1 || res.BuildOrder[0].ProjectRecID != "distribute" {
		t.Fatalf("build order=%+v", res.BuildOrder)
	}
}

func TestAnalyzeTarget_skipUsesRequiredVersion(t *testing.T) {
	dir := t.TempDir()
	repo := filepath.Join(dir, "repo")
	_ = os.MkdirAll(filepath.Join(repo, "com", "flux", "wmsv6", "flux-collaboration-storage", "V10.0.0"), 0o755)
	oldJar := filepath.Join(repo, "com", "flux", "wmsv6", "flux-collaboration-storage", "V10.0.0", "flux-collaboration-storage-V10.0.0.jar")
	if err := os.WriteFile(oldJar, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	modules := []Module{
		{
			ProjectRecID: "storage",
			ProjectName:  "flux-collaboration-storage",
			Coordinates:  Coordinates{GroupID: "com.flux.wmsv6", ArtifactID: "flux-collaboration-storage", Version: "V10.0.0"},
		},
		{
			ProjectRecID: "timer",
			ProjectName:  "flux-collaboration-timer",
			Dependencies: []Coordinates{
				{GroupID: "com.flux.wmsv6", ArtifactID: "flux-collaboration-storage", Version: "V10.0.0-P01"},
			},
			Coordinates: Coordinates{GroupID: "com.flux.wmsv6", ArtifactID: "flux-collaboration-timer", Version: "V10.0.0-P01"},
		},
	}
	ws := NewWorkspace(modules, repo)
	res, err := ws.AnalyzeTarget("timer")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.BuildOrder) != 1 || len(res.SkippedInstalled) != 0 {
		t.Fatalf("build=%d skipped=%d", len(res.BuildOrder), len(res.SkippedInstalled))
	}
}
