package packrun

import (
	"strings"
	"sync"
)

// Per-project mutex: one build or upstream install at a time per project directory.
type projectBuildGate struct {
	mu sync.Mutex
}

type projectBuildLocker struct {
	gates sync.Map // project rec_id -> *projectBuildGate
}

func (l *projectBuildLocker) gate(recID string) *projectBuildGate {
	recID = strings.TrimSpace(recID)
	if recID == "" {
		return &projectBuildGate{}
	}
	if g, ok := l.gates.Load(recID); ok {
		return g.(*projectBuildGate)
	}
	g := &projectBuildGate{}
	actual, _ := l.gates.LoadOrStore(recID, g)
	return actual.(*projectBuildGate)
}

// tryAcquireProjectBuild locks a project for build/install; returns false when already busy.
func (r *Runner) tryAcquireProjectBuild(projectRecID string) (release func(), ok bool) {
	g := r.projectLocks.gate(projectRecID)
	if !g.mu.TryLock() {
		return nil, false
	}
	return g.mu.Unlock, true
}

// acquireProjectBuild blocks until the project is free, then locks it.
func (r *Runner) acquireProjectBuild(projectRecID string) (release func()) {
	g := r.projectLocks.gate(projectRecID)
	g.mu.Lock()
	return g.mu.Unlock
}

// acquireProjectBuildWithLog tries the lock first; on contention it logs then blocks.
func (r *Runner) acquireProjectBuildWithLog(
	projectRecID, projectName string,
	waiting func(format string, args ...any),
	acquired func(format string, args ...any),
) (release func()) {
	if release, ok := r.tryAcquireProjectBuild(projectRecID); ok {
		return release
	}
	if waiting != nil {
		waiting("Waiting for project lock: %s (another build or upstream install in progress)", projectName)
	}
	release = r.acquireProjectBuild(projectRecID)
	if acquired != nil {
		acquired("Project lock acquired: %s", projectName)
	}
	return release
}
