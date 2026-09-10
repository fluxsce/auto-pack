package access

import (
	"strings"

	"github.com/google/uuid"
)

// NewID returns a UUID v4 string.
func NewID() string {
	return uuid.NewString()
}

const (
	// DefaultAdminRecID is the seeded admin user.
	DefaultAdminRecID = "00000000-0000-4000-8000-000000000001"
	// DefaultWorkspaceRecID is the seeded default workspace.
	DefaultWorkspaceRecID = "00000000-0000-4000-8000-000000000010"
	// GlobalMemberWorkspaceID is the membership row that is not bound to a workspace.
	GlobalMemberWorkspaceID = ""
)

// WorkspaceOrDefault returns id, or the seeded default workspace when blank.
func WorkspaceOrDefault(id string) string {
	id = strings.TrimSpace(id)
	if id == "" {
		return DefaultWorkspaceRecID
	}
	return id
}
