package record

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestCoordinatorSingleFlight(t *testing.T) {
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
	if err := c.Start(context.Background(), "ws-b", "B", 1, Job{ReleaseRecID: "r2"}, func(context.Context, *Job) {}); err != ErrReleaseInProgress {
		t.Fatalf("got %v", err)
	}
	st := c.Status("ws-b")
	if !st.Foreign || st.ActiveWorkspace != "A" {
		t.Fatalf("%+v", st)
	}
	close(block)
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if c.Status("ws-a").Status == JobStatusCompleted {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("job did not complete")
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
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if c.Status("ws").Status == JobStatusCompleted {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("panic left coordinator running")
}
