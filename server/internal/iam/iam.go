// Package iam 横切身份、空间成员与动作。Runner 不得调用本包。
package iam

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// Action is a named write or privileged read.
type Action string

const (
	ActionJobSubmit     Action = "job.submit"
	ActionJobCancel     Action = "job.cancel"
	ActionProjectWrite  Action = "project.write"
	ActionEnvWrite      Action = "env.write"
	ActionUserAdmin     Action = "user.admin"
	ActionSettingsWrite Action = "settings.write"
	ActionAuditRead     Action = "audit.read"
)

// Role codes stored on AP_WORKSPACE_MEMBER.
const (
	RoleOwner    = "owner"
	RoleOperator = "operator"
	RoleViewer   = "viewer"
)

var (
	ErrUnauthenticated = errors.New("iam: unauthenticated")
	ErrForbidden       = errors.New("iam: forbidden")
)

// Actor is the authenticated caller. JWT only proves identity.
type Actor struct {
	UserID    string
	LoginName string
}

// Member is one workspace membership row.
type Member struct {
	WorkspaceID string
	UserID      string
	Role        string
}

// Directory looks up membership and role grants. Implemented by user/workspace DAOs.
type Directory interface {
	FindMember(ctx context.Context, workspaceID, userID string) (Member, error)
	RoleAllows(ctx context.Context, role string, action Action) (bool, error)
	IsEnabledUser(ctx context.Context, userID string) (bool, error)
}

// Guard is the 2.0 permission gate. Every write service must call Allow.
type Guard struct {
	dir Directory
}

// NewGuard returns a Guard backed by dir.
func NewGuard(dir Directory) *Guard {
	return &Guard{dir: dir}
}

// Allow checks identity, workspace membership, and role action.
// user.admin is global: any workspace where the actor is owner, or the default workspace owner.
func (g *Guard) Allow(ctx context.Context, actor Actor, workspaceID string, action Action) error {
	if g == nil || g.dir == nil {
		return fmt.Errorf("iam: guard not configured")
	}
	if strings.TrimSpace(actor.UserID) == "" {
		return ErrUnauthenticated
	}
	ok, err := g.dir.IsEnabledUser(ctx, actor.UserID)
	if err != nil {
		return err
	}
	if !ok {
		return ErrForbidden
	}
	if action == ActionUserAdmin || action == ActionSettingsWrite {
		return g.allowOwnerAction(ctx, actor, workspaceID, action)
	}
	ws := strings.TrimSpace(workspaceID)
	if ws == "" {
		return fmt.Errorf("%w: workspace required", ErrForbidden)
	}
	m, err := g.dir.FindMember(ctx, ws, actor.UserID)
	if err != nil {
		return ErrForbidden
	}
	allowed, err := g.dir.RoleAllows(ctx, m.Role, action)
	if err != nil {
		return err
	}
	if !allowed {
		return fmt.Errorf("%w: %s", ErrForbidden, action)
	}
	return nil
}

func (g *Guard) allowOwnerAction(ctx context.Context, actor Actor, workspaceID string, action Action) error {
	ok, err := g.dir.RoleAllows(ctx, RoleOwner, action)
	if err != nil {
		return err
	}
	if !ok {
		return ErrForbidden
	}
	if ws := strings.TrimSpace(workspaceID); ws != "" {
		m, err := g.dir.FindMember(ctx, ws, actor.UserID)
		if err == nil && m.Role == RoleOwner {
			return nil
		}
	}
	m, err := g.dir.FindMember(ctx, workspaceID, actor.UserID)
	if err != nil {
		return ErrForbidden
	}
	if m.Role != RoleOwner {
		return ErrForbidden
	}
	return nil
}

// CanRead reports whether the actor may see workspace data (any membership).
func (g *Guard) CanRead(ctx context.Context, actor Actor, workspaceID string) error {
	if strings.TrimSpace(actor.UserID) == "" {
		return ErrUnauthenticated
	}
	ok, err := g.dir.IsEnabledUser(ctx, actor.UserID)
	if err != nil {
		return err
	}
	if !ok {
		return ErrForbidden
	}
	_, err = g.dir.FindMember(ctx, workspaceID, actor.UserID)
	if err != nil {
		return ErrForbidden
	}
	return nil
}
