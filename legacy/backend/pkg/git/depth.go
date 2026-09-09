package git

import (
	"fmt"
	"sync/atomic"
)

// DefaultCloneDepth is the recommended shallow history length for autopack.
//
// Why 50 (not 1): packing only needs the tip tree, but release flows often need
// a short window of recent commits — for example creating a release branch near
// HEAD, comparing recent history, or recovering from a force-pushed tip.
// Depth 1 maximizes clone speed but breaks more easily when a needed commit is
// not the very tip; 50 is a practical balance for large histories that still
// contain bulky blobs (e.g. formerly committed target/*.jar).
//
// Risks / limits (operators should read before lowering this value):
//   - Commits older than N on the tracked branch are absent locally until
//     deepen/unshallow; git log / merge-base against ancient history may fail.
//   - Creating a release branch from a very old tag is OK only because tag
//     fetch paths intentionally omit --depth (see EnsureTag / release fetch).
//   - Setting 0 disables shallow clone/fetch (full history; slow on fat repos).
const DefaultCloneDepth = 50

// globalCloneDepth stores the process-wide clone/fetch depth from app.yaml.
// 0 means full history; unset/negative values are treated as DefaultCloneDepth
// at SetGlobalCloneDepth time.
var globalCloneDepth atomic.Int64

func init() {
	globalCloneDepth.Store(int64(DefaultCloneDepth))
}

// SetGlobalCloneDepth applies configs/app.yaml git.clone_depth for this process.
//
//	depth > 0  → use shallow clone/fetch with that depth
//	depth == 0 → full history (no --depth)
//	depth < 0  → fall back to DefaultCloneDepth
func SetGlobalCloneDepth(depth int) {
	if depth < 0 {
		depth = DefaultCloneDepth
	}
	globalCloneDepth.Store(int64(depth))
}

// GlobalCloneDepth returns the effective process-wide depth (0 = full history).
func GlobalCloneDepth() int {
	return int(globalCloneDepth.Load())
}

// AppendCloneDepthArgs appends git clone --depth N when shallow mode is enabled.
func AppendCloneDepthArgs(args []string) []string {
	if d := GlobalCloneDepth(); d > 0 {
		return append(args, "--depth", fmt.Sprintf("%d", d))
	}
	return args
}

// AppendFetchDepthArgs appends git fetch --depth=N when shallow mode is enabled.
// Prefer this on branch tip sync. Do not use on on-demand version-tag fetches
// that must reach tags outside the shallow window of the current branch tip.
func AppendFetchDepthArgs(args []string) []string {
	if d := GlobalCloneDepth(); d > 0 {
		return append(args, fmt.Sprintf("--depth=%d", d))
	}
	return args
}
