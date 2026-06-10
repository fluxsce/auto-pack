package packrun

import (
	"sync"
	"testing"
	"time"
)

func TestProjectBuildLock_serializesSameProject(t *testing.T) {
	r := &Runner{}
	release, ok := r.tryAcquireProjectBuild("proj-a")
	if !ok || release == nil {
		t.Fatal("expected first acquire ok")
	}
	_, ok2 := r.tryAcquireProjectBuild("proj-a")
	if ok2 {
		t.Fatal("expected second acquire to fail")
	}
	release()

	_, ok3 := r.tryAcquireProjectBuild("proj-a")
	if !ok3 {
		t.Fatal("expected acquire after release")
	}
}

func TestProjectBuildLock_acquireBlocksUntilRelease(t *testing.T) {
	r := &Runner{}
	release, ok := r.tryAcquireProjectBuild("proj-b")
	if !ok {
		t.Fatal("acquire")
	}

	done := make(chan struct{})
	go func() {
		unlock := r.acquireProjectBuild("proj-b")
		unlock()
		close(done)
	}()

	select {
	case <-done:
		t.Fatal("expected acquire to block")
	case <-time.After(80 * time.Millisecond):
	}

	release()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("expected acquire to complete after release")
	}
}

func TestProjectBuildLock_independentProjects(t *testing.T) {
	r := &Runner{}
	a, ok1 := r.tryAcquireProjectBuild("a")
	b, ok2 := r.tryAcquireProjectBuild("b")
	if !ok1 || !ok2 {
		t.Fatal("expected independent locks")
	}
	a()
	b()
}

// Simulates two parallel builds that each need the other's project lock for upstream
// install (circular Maven deps). Without holding self-lock during prereq, one side
// finishes installing the other and both complete instead of deadlocking.
func TestProjectBuildLock_noDeadlockOnCircularUpstreamWait(t *testing.T) {
	r := &Runner{}

	done := make(chan struct{})
	go func() {
		releaseA := r.acquireProjectBuild("proj-a")
		releaseA()
		releaseB := r.acquireProjectBuild("proj-b")
		releaseB()
		close(done)
	}()

	releaseB := r.acquireProjectBuild("proj-b")
	releaseB()
	releaseA := r.acquireProjectBuild("proj-a")
	releaseA()

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("expected circular upstream wait to resolve without deadlock")
	}
}

// Simulates two pipeline builds each syncing the same workspace project list (A..C).
// Each build only holds one project lock at a time; must not deadlock.
func TestProjectBuildLock_parallelWorkspaceSync(t *testing.T) {
	r := &Runner{}
	order := []string{"srvapi", "adapter", "agent"}

	syncWorkspace := func() {
		for _, id := range order {
			release := r.acquireProjectBuild(id)
			time.Sleep(2 * time.Millisecond)
			release()
		}
	}

	done := make(chan struct{})
	go func() {
		syncWorkspace()
		close(done)
	}()
	syncWorkspace()

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("parallel workspace sync deadlocked")
	}
}

func TestProjectBuildLock_concurrentTryAcquire(t *testing.T) {
	r := &Runner{}
	const n = 16
	var wg sync.WaitGroup
	var won int32
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			release := r.acquireProjectBuild("shared")
			time.Sleep(5 * time.Millisecond)
			release()
			won++
		}()
	}
	wg.Wait()
	if won != n {
		t.Fatalf("all goroutines should eventually acquire; got %d serial wins", won)
	}
}
