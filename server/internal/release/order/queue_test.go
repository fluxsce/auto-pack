package order

import (
	"testing"

	"auto-pack/server/internal/project"
)

func TestQueueSelectedPullsUpstream(t *testing.T) {
	list := []project.Project{
		{RecID: "a", ProjectName: "A", ProjectType: "maven", IsEnabled: 1},
		{RecID: "b", ProjectName: "B", ProjectType: "maven", IsEnabled: 1},
		{RecID: "c", ProjectName: "C", ProjectType: "go", IsEnabled: 1},
	}
	analysis := MavenAnalysis{
		FullBuildOrder: []MavenModule{{ProjectRecID: "a"}, {ProjectRecID: "b"}},
		InternalEdges:  []MavenEdge{{ConsumerRecID: "b", ProviderRecID: "a"}},
	}
	got := Queue(list, analysis, QueueOpts{TargetRecIDs: []string{"b"}})
	if len(got) != 2 || got[0].RecID != "a" || got[1].RecID != "b" {
		t.Fatalf("%+v", got)
	}
}

func TestQueueWorkspaceIncludesOthers(t *testing.T) {
	list := []project.Project{
		{RecID: "a", ProjectName: "A", ProjectType: "maven", IsEnabled: 1},
		{RecID: "c", ProjectName: "C", ProjectType: "go", IsEnabled: 1},
		{RecID: "d", ProjectName: "D", ProjectType: "maven", IsEnabled: 0},
	}
	analysis := MavenAnalysis{FullBuildOrder: []MavenModule{{ProjectRecID: "a"}}}
	got := Queue(list, analysis, QueueOpts{IncludeOthers: true})
	if len(got) != 2 || got[0].RecID != "a" || got[1].RecID != "c" {
		t.Fatalf("%+v", got)
	}
}
