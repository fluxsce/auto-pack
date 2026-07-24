package packrun

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestWorkspaceSourceSync_parallelDifferentWorkspaces(t *testing.T) {
	prev := WorkspaceSyncMaxConcurrent()
	SetWorkspaceSyncMaxConcurrent(2)
	t.Cleanup(func() { SetWorkspaceSyncMaxConcurrent(prev) })

	var c workspaceSourceSyncCoordinator
	var wg sync.WaitGroup
	started := make(chan string, 2)
	release := make(chan struct{})

	startHeld := func(id string) {
		wg.Add(1)
		err := c.start(id, "WS-"+id, func(ctx context.Context, job *WorkspaceSourceSyncJob) {
			defer wg.Done()
			started <- id
			<-release
		})
		if err != nil {
			t.Errorf("start %s: %v", id, err)
		}
	}

	startHeld("ws-a")
	startHeld("ws-b")

	got := map[string]bool{}
	deadline := time.After(2 * time.Second)
	for len(got) < 2 {
		select {
		case id := <-started:
			got[id] = true
		case <-deadline:
			t.Fatalf("expected both workspaces to start; got %v", got)
		}
	}

	// Same workspace while still running is idempotent.
	secondStarted := make(chan struct{})
	if err := c.start("ws-a", "WS-A", func(ctx context.Context, job *WorkspaceSourceSyncJob) {
		close(secondStarted)
	}); err != nil {
		t.Fatal(err)
	}
	select {
	case <-secondStarted:
		t.Fatal("same workspace should not start a second runner")
	case <-time.After(50 * time.Millisecond):
	}

	close(release)
	wg.Wait()
}

func TestWorkspaceSourceSync_capacityLimit(t *testing.T) {
	prev := WorkspaceSyncMaxConcurrent()
	SetWorkspaceSyncMaxConcurrent(2)
	t.Cleanup(func() { SetWorkspaceSyncMaxConcurrent(prev) })

	var c workspaceSourceSyncCoordinator
	gate := make(chan struct{})
	hold := func(ctx context.Context, job *WorkspaceSourceSyncJob) { <-gate }

	if err := c.start("a", "A", hold); err != nil {
		t.Fatal(err)
	}
	if err := c.start("b", "B", hold); err != nil {
		t.Fatal(err)
	}
	if err := c.start("c", "C", hold); err != ErrWorkspaceSyncCapacity {
		t.Fatalf("want capacity error, got %v", err)
	}
	running, max, names := c.RunningSyncSummary()
	if running != 2 || max != 2 {
		t.Fatalf("summary running=%d max=%d names=%v", running, max, names)
	}
	close(gate)
}

func TestWorkspaceSourceSync_statusIsPerWorkspace(t *testing.T) {
	prev := WorkspaceSyncMaxConcurrent()
	SetWorkspaceSyncMaxConcurrent(2)
	t.Cleanup(func() { SetWorkspaceSyncMaxConcurrent(prev) })

	var c workspaceSourceSyncCoordinator
	var wg sync.WaitGroup
	wg.Add(1)
	gate := make(chan struct{})
	_ = c.start("a", "A", func(ctx context.Context, job *WorkspaceSourceSyncJob) {
		defer wg.Done()
		<-gate
	})

	stA := c.status("a")
	if stA.Status != SyncJobStatusRunning || stA.Foreign {
		t.Fatalf("status a: %+v", stA)
	}
	stB := c.status("b")
	if stB.Status != SyncJobStatusIdle || stB.Foreign {
		// Other workspace must see idle for itself — not a foreign global lock.
		t.Fatalf("status b should be idle (not foreign): %+v", stB)
	}
	close(gate)
	wg.Wait()
}

func TestSetWorkspaceSyncMaxConcurrent(t *testing.T) {
	prev := WorkspaceSyncMaxConcurrent()
	t.Cleanup(func() { SetWorkspaceSyncMaxConcurrent(prev) })

	SetWorkspaceSyncMaxConcurrent(0)
	if WorkspaceSyncMaxConcurrent() != DefaultWorkspaceSyncMaxConcurrent {
		t.Fatalf("got %d", WorkspaceSyncMaxConcurrent())
	}
	SetWorkspaceSyncMaxConcurrent(4)
	if WorkspaceSyncMaxConcurrent() != 4 {
		t.Fatalf("got %d", WorkspaceSyncMaxConcurrent())
	}
}
