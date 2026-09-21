package batchrun

import (
	"context"
	"sync"
	"testing"
	"time"

	"auto-pack/server/internal/project"
	ord "auto-pack/server/internal/release/order"
)

func TestRunGraphWaitsForUpstream(t *testing.T) {
	queue := []project.Project{
		{RecID: "a", ProjectName: "A"},
		{RecID: "b", ProjectName: "B"},
	}
	analysis := ord.MavenAnalysis{
		InternalEdges: []ord.MavenEdge{{ConsumerRecID: "b", ProviderRecID: "a"}},
	}
	releaseA := make(chan struct{})
	var mu sync.Mutex
	var orderIDs []string
	build := func(_ context.Context, prj project.Project) Outcome {
		mu.Lock()
		orderIDs = append(orderIDs, prj.RecID)
		mu.Unlock()
		if prj.RecID == "a" {
			<-releaseA
		}
		return Outcome{OK: true}
	}
	doneCh := make(chan map[string]string, 1)
	go func() {
		doneCh <- RunGraph(context.Background(), queue, analysis, build, GraphHooks{})
	}()
	deadline := time.Now().Add(2 * time.Second)
	for {
		mu.Lock()
		n := len(orderIDs)
		first := ""
		if n > 0 {
			first = orderIDs[0]
		}
		mu.Unlock()
		if n >= 1 {
			if first != "a" {
				t.Fatalf("first start=%s", first)
			}
			if n != 1 {
				t.Fatalf("B started before A finished: %v", orderIDs)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("A did not start")
		}
		time.Sleep(10 * time.Millisecond)
	}
	close(releaseA)
	got := <-doneCh
	if got["a"] != ord.OutcomeOK || got["b"] != ord.OutcomeOK {
		t.Fatal(got)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(orderIDs) != 2 || orderIDs[1] != "b" {
		t.Fatalf("order=%v", orderIDs)
	}
}

func TestRunGraphSkipsFailedUpstream(t *testing.T) {
	queue := []project.Project{
		{RecID: "a", ProjectName: "A"},
		{RecID: "b", ProjectName: "B"},
	}
	analysis := ord.MavenAnalysis{
		InternalEdges: []ord.MavenEdge{{ConsumerRecID: "b", ProviderRecID: "a"}},
	}
	var skipped string
	got := RunGraph(context.Background(), queue, analysis, func(_ context.Context, prj project.Project) Outcome {
		if prj.RecID == "a" {
			return Outcome{OK: false, Message: "boom"}
		}
		t.Fatal("B should not build")
		return Outcome{OK: true}
	}, GraphHooks{OnSkip: func(prj project.Project, reason string) {
		skipped = prj.RecID + ":" + reason
	}})
	if got["a"] != ord.OutcomeFailed || got["b"] != ord.OutcomeSkipped {
		t.Fatal(got)
	}
	if skipped == "" {
		t.Fatal("expected skip hook")
	}
}

func TestRunGraphIndependentInParallel(t *testing.T) {
	queue := []project.Project{
		{RecID: "a", ProjectName: "A"},
		{RecID: "c", ProjectName: "C"},
	}
	started := make(chan string, 2)
	release := make(chan struct{})
	build := func(_ context.Context, prj project.Project) Outcome {
		started <- prj.RecID
		<-release
		return Outcome{OK: true}
	}
	doneCh := make(chan map[string]string, 1)
	go func() {
		doneCh <- RunGraph(context.Background(), queue, ord.MavenAnalysis{}, build, GraphHooks{})
	}()
	seen := map[string]bool{}
	deadline := time.After(2 * time.Second)
	for len(seen) < 2 {
		select {
		case id := <-started:
			seen[id] = true
		case <-deadline:
			t.Fatalf("parallel start timeout, seen=%v", seen)
		}
	}
	close(release)
	got := <-doneCh
	if got["a"] != ord.OutcomeOK || got["c"] != ord.OutcomeOK {
		t.Fatal(got)
	}
}

func TestRunGraphCapsInflight(t *testing.T) {
	queue := []project.Project{
		{RecID: "a", ProjectName: "A"},
		{RecID: "b", ProjectName: "B"},
		{RecID: "c", ProjectName: "C"},
	}
	var mu sync.Mutex
	inflight, maxSeen := 0, 0
	started := make(chan struct{}, 3)
	release := make(chan struct{})
	build := func(_ context.Context, _ project.Project) Outcome {
		mu.Lock()
		inflight++
		if inflight > maxSeen {
			maxSeen = inflight
		}
		mu.Unlock()
		started <- struct{}{}
		<-release
		mu.Lock()
		inflight--
		mu.Unlock()
		return Outcome{OK: true}
	}
	doneCh := make(chan map[string]string, 1)
	go func() {
		doneCh <- RunGraph(context.Background(), queue, ord.MavenAnalysis{}, build, GraphHooks{MaxInflight: 1})
	}()
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("first project did not start")
	}
	select {
	case <-started:
		t.Fatal("second project started while first still running")
	case <-time.After(80 * time.Millisecond):
	}
	close(release)
	got := <-doneCh
	if got["a"] != ord.OutcomeOK || got["b"] != ord.OutcomeOK || got["c"] != ord.OutcomeOK {
		t.Fatal(got)
	}
	mu.Lock()
	defer mu.Unlock()
	if maxSeen != 1 {
		t.Fatalf("max inflight=%d want 1", maxSeen)
	}
}

func TestRunGraphHookPanicDoesNotHang(t *testing.T) {
	queue := []project.Project{
		{RecID: "a", ProjectName: "A"},
		{RecID: "b", ProjectName: "B"},
	}
	got := RunGraph(context.Background(), queue, ord.MavenAnalysis{}, func(_ context.Context, _ project.Project) Outcome {
		return Outcome{OK: true}
	}, GraphHooks{
		OnStart:  func(project.Project) { panic("start") },
		OnFinish: func(project.Project, Outcome, time.Duration) { panic("finish") },
	})
	if got["a"] != ord.OutcomeOK || got["b"] != ord.OutcomeOK {
		t.Fatal(got)
	}
}
