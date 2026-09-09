package mavendeps

import (
	"strings"
	"testing"
)

func TestVersionsEquivalent_snapshotRelease(t *testing.T) {
	if !versionsEquivalent("V10.0.0-P01-SNAPSHOT", "V10.0.0-P01") {
		t.Fatal("snapshot/release pair should align")
	}
	if !versionsEquivalent("V10.0.0-P01", "V10.0.0-P01-SNAPSHOT") {
		t.Fatal("release/snapshot pair should align")
	}
	if versionsEquivalent("V10.0.0-P01-SNAPSHOT", "develop") {
		t.Fatal("develop should not align with snapshot release line")
	}
}

func TestWorkspaceUpstreamMatch_legacyProviderSnapshotWithVReleaseDep(t *testing.T) {
	modules := []Module{
		{
			ProjectRecID:   "api",
			ProjectName:    "flux-collaboration-datahub01api",
			ProjectVersion: "develop",
			Coordinates: Coordinates{
				GroupID: "com.flux.wmsv6", ArtifactID: "flux-collaboration-datahub01api",
				Version: "10.0.0-SNAPSHOT",
			},
		},
		{
			ProjectRecID:   "srv",
			ProjectName:    "flux-collaboration-datahub01srv",
			ProjectVersion: "develop",
			Dependencies: []Coordinates{
				{GroupID: "com.flux.wmsv6", ArtifactID: "flux-collaboration-datahub01api", Version: "V10.0.0-P01-SNAPSHOT"},
			},
			Coordinates: Coordinates{
				GroupID: "com.flux.wmsv6", ArtifactID: "flux-collaboration-datahub01srv",
				Version: "V10.0.0-P01-SNAPSHOT",
			},
		},
	}
	ws := NewWorkspace(modules, t.TempDir())
	unsatisfied := ws.ValidateTargetDependencies("srv")
	if len(unsatisfied) != 1 {
		t.Fatalf("unsatisfied=%+v", unsatisfied)
	}
	if strings.Contains(unsatisfied[0].Reason, "版本未对齐") {
		t.Fatalf("expected missing install, got %q", unsatisfied[0].Reason)
	}
	res, err := ws.AnalyzeTarget("srv")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.BuildOrder) != 1 || res.BuildOrder[0].ProjectRecID != "api" {
		t.Fatalf("build order=%+v", res.BuildOrder)
	}
}

func TestWorkspaceUpstreamMatch_legacyDepVersionWithVReleaseProvider(t *testing.T) {
	modules := []Module{
		{
			ProjectRecID:   "cache",
			ProjectName:    "flux-datahub-cache",
			ProjectVersion: "V9.0.0-P06-02",
			Coordinates: Coordinates{
				GroupID: "com.flux.datahub", ArtifactID: "flux-datahub-cache",
				Version: "V9.0.0-P06-02-SNAPSHOT",
			},
		},
		{
			ProjectRecID:   "dataop",
			ProjectName:    "flux-collaboration-storage-dataoperation",
			ProjectVersion: "V9.0.0-P06-02",
			Dependencies: []Coordinates{
				{GroupID: "com.flux.datahub", ArtifactID: "flux-datahub-cache", Version: "9.0.0-SNAPSHOT"},
			},
			Coordinates: Coordinates{
				GroupID: "com.flux.wmsv6", ArtifactID: "flux-collaboration-storage-dataoperation",
				Version: "V9.0.0-P06-02-SNAPSHOT",
			},
		},
	}
	ws := NewWorkspace(modules, t.TempDir())
	unsatisfied := ws.ValidateTargetDependencies("dataop")
	if len(unsatisfied) != 1 {
		t.Fatalf("unsatisfied=%+v", unsatisfied)
	}
	if strings.Contains(unsatisfied[0].Reason, "版本未对齐") {
		t.Fatalf("expected missing install, got %q", unsatisfied[0].Reason)
	}
}

func TestValidateTargetDependencies_ignoresProjectVersionLabel(t *testing.T) {
	modules := []Module{
		{
			ProjectRecID:   "srv",
			ProjectName:    "flux-collaboration-datahub01srv",
			ProjectVersion: "develop",
			Coordinates: Coordinates{
				GroupID: "com.flux.wmsv6", ArtifactID: "flux-collaboration-datahub01srv",
				Version: "V10.0.0-P01-SNAPSHOT",
			},
		},
		{
			ProjectRecID:   "agent",
			ProjectName:    "flux-collaboration-agent",
			ProjectVersion: "develop",
			Dependencies: []Coordinates{
				{GroupID: "com.flux.wmsv6", ArtifactID: "flux-collaboration-datahub01srv", Version: "V10.0.0-P01-SNAPSHOT"},
			},
			Coordinates: Coordinates{
				GroupID: "com.flux.wmsv6", ArtifactID: "flux-collaboration-agent",
				Version: "V10.0.0-P01-SNAPSHOT",
			},
		},
	}
	ws := NewWorkspace(modules, t.TempDir())
	unsatisfied := ws.ValidateTargetDependencies("agent")
	if len(unsatisfied) != 1 {
		t.Fatalf("unsatisfied=%+v", unsatisfied)
	}
	if unsatisfied[0].Reason == "" || strings.Contains(unsatisfied[0].Reason, "版本未对齐") {
		t.Fatalf("expected missing upstream, got %q", unsatisfied[0].Reason)
	}
}
