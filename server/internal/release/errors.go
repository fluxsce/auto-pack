package release

import (
	"errors"

	"auto-pack/server/internal/release/record"
)

var (
	// ErrReleaseInProgress is returned when another auto-release is already running.
	ErrReleaseInProgress = record.ErrReleaseInProgress
	// ErrNotFound is returned when the release record does not exist in this workspace.
	ErrNotFound = record.ErrNotFound
	// ErrNoActive is returned when post-options are saved without a running release.
	ErrNoActive = record.ErrNoActive
	// ErrNoProjects is returned when the workspace has nothing to release.
	ErrNoProjects = errors.New("workspace has no projects")
)
