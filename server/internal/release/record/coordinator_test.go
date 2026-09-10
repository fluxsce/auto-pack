package record

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestCoordinatorSameWorkspaceSingleFlight(t *testing.T) {
	c := NewCoordinator()
	started := make(chan struct{})
	block := make(chan struct{})
	if err := c.Start(context.Background(), "ws-a", "A", 1, Job{ReleaseRecID: "r1"}, func(ctx context.Context, job *Job) {
		close(started)
		<-block
	}); err != nil {
		t.Fatal(err)
	}
	<-started
	if err := c.Start(context.Background(), "ws-a", "A", 1, Job{ReleaseRecID: "r2"}, func(context.Context, *Job) {}); err != ErrReleaseInProgress {
		t.Fatalf("same workspace should reject, got %v", err)
	}
	close(block)
	waitCompleted(t, c, "ws-a")
}

func TestCoordinatorPerWorkspaceParallel(t *testing.T) {
	c := NewCoordinator()
	startedA := make(chan struct{})
	startedB := make(chan struct{})
	blockA := make(chan struct{})
	blockB := make(chan struct{})
	if err := c.Start(context.Background(), "ws-a", "A", 2, Job{ReleaseRecID: "r1"}, func(ctx context.Context, job *Job) {
		close(startedA)
		<-blockA
	}); err != nil {
		t.Fatal(err)
	}
	if err := c.Start(context.Background(), "ws-b", "B", 3, Job{ReleaseRecID: "r2"}, func(ctx context.Context, job *Job) {
		close(startedB)
		<-blockB
	}); err != nil {
		t.Fatalf("other workspace should start, got %v", err)
	}
	<-startedA
	<-startedB
	stA := c.Status("ws-a")
	if stA.Status != JobStatusRunning || stA.Foreign || stA.ReleaseRecID != "r1" {
		t.Fatalf("status a: %+v", stA)
	}
	stB := c.Status("ws-b")
	if stB.Status != JobStatusRunning || stB.Foreign || stB.ReleaseRecID != "r2" {
		t.Fatalf("status b: %+v", stB)
	}
	stC := c.Status("ws-c")
	if stC.Status != JobStatusIdle || stC.Foreign {
		t.Fatalf("idle workspace must not see others: %+v", stC)
	}
	close(blockA)
	close(blockB)
	waitCompleted(t, c, "ws-a")
	waitCompleted(t, c, "ws-b")
}

func TestCoordinatorProgressStaysOnJob(t *testing.T) {
	c := NewCoordinator()
	startedA := make(chan struct{})
	startedB := make(chan struct{})
	block := make(chan struct{})
	var jobA, jobB *Job
	if err := c.Start(context.Background(), "ws-a", "A", 2, Job{ReleaseRecID: "r1"}, func(ctx context.Context, job *Job) {
		jobA = job
		close(startedA)
		<-block
	}); err != nil {
		t.Fatal(err)
	}
	if err := c.Start(context.Background(), "ws-b", "B", 2, Job{ReleaseRecID: "r2"}, func(ctx context.Context, job *Job) {
		jobB = job
		close(startedB)
		<-block
	}); err != nil {
		t.Fatal(err)
	}
	<-startedA
	<-startedB
	c.SetProgress(jobA, "proj-a", 1)
	c.SetProgress(jobB, "proj-b", 0)
	if got := c.Status("ws-a"); got.CurrentProject != "proj-a" || got.DoneCount != 1 {
		t.Fatalf("a progress: %+v", got)
	}
	if got := c.Status("ws-b"); got.CurrentProject != "proj-b" || got.DoneCount != 0 {
		t.Fatalf("b progress: %+v", got)
	}
	close(block)
	waitCompleted(t, c, "ws-a")
	waitCompleted(t, c, "ws-b")
}

func TestCoordinatorRecoversPanic(t *testing.T) {
	c := NewCoordinator()
	var wg sync.WaitGroup
	wg.Add(1)
	if err := c.Start(context.Background(), "ws", "W", 1, Job{ReleaseRecID: "r"}, func(context.Context, *Job) {
		defer wg.Done()
		panic("boom")
	}); err != nil {
		t.Fatal(err)
	}
	wg.Wait()
	waitCompleted(t, c, "ws")
}

func waitCompleted(t *testing.T, c *Coordinator, workspaceID string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if c.Status(workspaceID).Status == JobStatusCompleted {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("job %s did not complete", workspaceID)
}
