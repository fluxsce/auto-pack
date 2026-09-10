package record

import "errors"

var (
	// ErrBuildInProgress is returned when this workspace already has an auto-build running.
	ErrBuildInProgress = errors.New("auto build already in progress")
	// ErrNotFound is returned when AP_WORKSPACE_BUILD has no matching row.
	ErrNotFound = errors.New("workspace build not found")
)
