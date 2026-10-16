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
	got, skipped := Queue(list, analysis, QueueOpts{TargetRecIDs: []string{"b"}})
	if len(got) != 2 || got[0].RecID != "a" || got[1].RecID != "b" || len(skipped) != 0 {
		t.Fatalf("%+v skipped=%v", got, skipped)
	}
}

func TestQueueExactTargetsSkipsSucceededUpstream(t *testing.T) {
	list := []project.Project{
		{RecID: "a", ProjectName: "A", ProjectType: "maven", IsEnabled: 1},
		{RecID: "b", ProjectName: "B", ProjectType: "maven", IsEnabled: 1},
		{RecID: "c", ProjectName: "C", ProjectType: "maven", IsEnabled: 1},
	}
	analysis := MavenAnalysis{
		FullBuildOrder: []MavenModule{{ProjectRecID: "a"}, {ProjectRecID: "b"}, {ProjectRecID: "c"}},
		InternalEdges: []MavenEdge{
			{ConsumerRecID: "b", ProviderRecID: "a"},
			{ConsumerRecID: "c", ProviderRecID: "b"},
		},
	}
	got, skipped := Queue(list, analysis, QueueOpts{TargetRecIDs: []string{"a", "c"}, ExactTargets: true})
	if len(got) != 2 || got[0].RecID != "a" || got[1].RecID != "c" || len(skipped) != 0 {
		t.Fatalf("%+v skipped=%v", got, skipped)
	}
}

func TestQueueWorkspaceIncludesOthers(t *testing.T) {
	list := []project.Project{
		{RecID: "a", ProjectName: "A", ProjectType: "maven", IsEnabled: 1},
		{RecID: "c", ProjectName: "C", ProjectType: "go", IsEnabled: 1},
		{RecID: "d", ProjectName: "D", ProjectType: "maven", IsEnabled: 0},
	}
	analysis := MavenAnalysis{FullBuildOrder: []MavenModule{{ProjectRecID: "a"}}}
	got, skipped := Queue(list, analysis, QueueOpts{IncludeOthers: true})
	if len(got) != 2 || got[0].RecID != "a" || got[1].RecID != "c" || len(skipped) != 0 {
		t.Fatalf("%+v skipped=%v", got, skipped)
	}
}

func TestQueueSkipsConsumerWhenUpstreamDisabled(t *testing.T) {
	list := []project.Project{
		{RecID: "a", ProjectName: "A", ProjectType: "maven", IsEnabled: 0},
		{RecID: "b", ProjectName: "B", ProjectType: "maven", IsEnabled: 1},
		{RecID: "c", ProjectName: "C", ProjectType: "maven", IsEnabled: 1},
	}
	analysis := MavenAnalysis{
		FullBuildOrder: []MavenModule{{ProjectRecID: "a"}, {ProjectRecID: "b"}, {ProjectRecID: "c"}},
		InternalEdges: []MavenEdge{
			{ConsumerRecID: "b", ConsumerName: "B", ProviderRecID: "a", ProviderName: "A"},
			{ConsumerRecID: "c", ConsumerName: "C", ProviderRecID: "b", ProviderName: "B"},
		},
	}
	got, skipped := Queue(list, analysis, QueueOpts{})
	if len(got) != 0 {
		t.Fatalf("want empty queue, got %+v", got)
	}
	if len(skipped) < 2 {
		t.Fatalf("skipped %+v", skipped)
	}
}
