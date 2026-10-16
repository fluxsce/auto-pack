package order

import (
	"testing"

	"auto-pack/server/internal/project"
)

func testQueueAB() []project.Project {
	return []project.Project{
		{RecID: "a", ProjectName: "A", ProjectType: "maven", IsEnabled: 1},
		{RecID: "b", ProjectName: "B", ProjectType: "maven", IsEnabled: 1},
		{RecID: "c", ProjectName: "C", ProjectType: "maven", IsEnabled: 1},
	}
}

func testAnalysisChain() MavenAnalysis {
	return MavenAnalysis{
		FullBuildOrder: []MavenModule{{ProjectRecID: "a"}, {ProjectRecID: "b"}, {ProjectRecID: "c"}},
		InternalEdges: []MavenEdge{
			{ConsumerRecID: "b", ProviderRecID: "a"},
			{ConsumerRecID: "c", ProviderRecID: "b"},
		},
	}
}

func TestGateReadyAfterUpstreamOK(t *testing.T) {
	g := NewGate(testQueueAB(), testAnalysisChain())
	ready, skip := g.Decide("a", nil)
	if !ready || skip != "" {
		t.Fatalf("a ready=%v skip=%s", ready, skip)
	}
	ready, skip = g.Decide("b", nil)
	if ready || skip != "" {
		t.Fatalf("b should wait, ready=%v skip=%s", ready, skip)
	}
	ready, skip = g.Decide("b", map[string]string{"a": OutcomeOK})
	if !ready || skip != "" {
		t.Fatalf("b after a ok: ready=%v skip=%s", ready, skip)
	}
}

func TestGateSkipWhenUpstreamFailed(t *testing.T) {
	g := NewGate(testQueueAB(), testAnalysisChain())
	ready, skip := g.Decide("b", map[string]string{"a": OutcomeFailed})
	if ready || skip == "" {
		t.Fatalf("b should skip, ready=%v skip=%s", ready, skip)
	}
	ready, skip = g.Decide("c", map[string]string{"a": OutcomeFailed, "b": OutcomeSkipped})
	if ready || skip == "" {
		t.Fatalf("c should skip, ready=%v skip=%s", ready, skip)
	}
}

func TestGateIgnoresProvidersOutsideQueue(t *testing.T) {
	queue := []project.Project{{RecID: "b", ProjectName: "B", ProjectType: "maven", IsEnabled: 1}}
	g := NewGate(queue, testAnalysisChain())
	ready, skip := g.Decide("b", nil)
	if !ready || skip != "" {
		t.Fatalf("exact target without A in queue must not block: ready=%v skip=%s", ready, skip)
	}
}

func TestQueueSelectedStillPullsUpstream(t *testing.T) {
	list := testQueueAB()
	got, _ := Queue(list, testAnalysisChain(), QueueOpts{TargetRecIDs: []string{"c"}})
	if len(got) != 3 || got[0].RecID != "a" || got[1].RecID != "b" || got[2].RecID != "c" {
		t.Fatalf("upstream expand lost: %+v", got)
	}
}
