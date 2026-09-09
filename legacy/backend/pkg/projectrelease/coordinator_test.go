package projectrelease

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

func TestCoordinator_singleFlight(t *testing.T) {
	c := NewCoordinator()
	var wg sync.WaitGroup
	wg.Add(1)
	err := c.Start("ws1", "WS1", 1, BatchReleaseJob{Tag: "v1", ReleaseBranch: "release/x"}, func(ctx context.Context, job *BatchReleaseJob) {
		wg.Wait()
		c.FinishResult(1, 0, 0, nil)
	})
	if err != nil {
		t.Fatal(err)
	}
	err = c.Start("ws2", "WS2", 1, BatchReleaseJob{}, func(context.Context, *BatchReleaseJob) {})
	if !errors.Is(err, ErrReleaseInProgress) {
		t.Fatalf("want ErrReleaseInProgress, got %v", err)
	}
	st := c.Status("ws2")
	if !st.Foreign || st.ActiveWorkspace != "WS1" {
		t.Fatalf("foreign status: %+v", st)
	}
	wg.Done()
	time.Sleep(50 * time.Millisecond)
}
