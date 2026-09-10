package build

import (
	"errors"

	"auto-pack/server/internal/build/record"
)

var (
	// ErrBuildInProgress is returned when this workspace already has an auto-build running.
	ErrBuildInProgress = record.ErrBuildInProgress
	// ErrNotFound is returned when the build record does not exist in this workspace.
	ErrNotFound = record.ErrNotFound
	// ErrNoProjects is returned when the workspace has nothing to build.
	ErrNoProjects = errors.New("workspace has no projects to build")
)
