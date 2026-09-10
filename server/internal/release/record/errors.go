package record

import "errors"

var (
	// ErrReleaseInProgress is returned when this workspace already has an auto-release running.
	ErrReleaseInProgress = errors.New("auto release already in progress")
	// ErrNotFound is returned when AP_WORKSPACE_RELEASE has no matching row.
	ErrNotFound = errors.New("workspace release not found")
	// ErrNoActive is returned when post-options are saved without a running release.
	ErrNoActive = errors.New("no active release")
)
