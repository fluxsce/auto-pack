package projectcfg

import (
	"errors"
	"regexp"
	"strings"
)

var workspaceCodePattern = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9._-]{0,31}$`)
var versionPattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]{0,31}$`)

func ValidateProjectName(name string) error {
	if strings.TrimSpace(name) == "" {
		return errors.New("project_name is required")
	}
	return nil
}

func ValidateVersion(version string) error {
	version = strings.TrimSpace(version)
	if version == "" {
		return errors.New("project_version is required")
	}
	if !versionPattern.MatchString(version) {
		return errors.New("project_version format is invalid")
	}
	return nil
}

func ValidateWorkspaceCode(code string) error {
	code = strings.TrimSpace(code)
	if code == "" {
		return errors.New("workspace_code is required")
	}
	if !workspaceCodePattern.MatchString(code) {
		return errors.New("workspace_code must start with a letter and contain only letters, digits, ., _ or -")
	}
	if strings.Contains(code, "..") {
		return errors.New("workspace_code cannot contain ..")
	}
	return nil
}
