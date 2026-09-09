package iam

import (
	"context"
	"errors"
	"testing"
)

type memDir struct {
	users   map[string]bool
	members map[string]Member
	grants  map[string]map[Action]bool
}

func (m *memDir) FindMember(_ context.Context, workspaceID, userID string) (Member, error) {
	v, ok := m.members[workspaceID+"/"+userID]
	if !ok {
		return Member{}, errors.New("not found")
	}
	return v, nil
}

func (m *memDir) RoleAllows(_ context.Context, role string, action Action) (bool, error) {
	if g, ok := m.grants[role]; ok {
		return g[action], nil
	}
	return false, nil
}

func (m *memDir) IsEnabledUser(_ context.Context, userID string) (bool, error) {
	return m.users[userID], nil
}

func testGuard() *Guard {
	return NewGuard(&memDir{
		users: map[string]bool{"u1": true, "u2": true, "off": false},
		members: map[string]Member{
			"ws1/u1": {WorkspaceID: "ws1", UserID: "u1", Role: RoleOwner},
			"ws1/u2": {WorkspaceID: "ws1", UserID: "u2", Role: RoleViewer},
		},
		grants: map[string]map[Action]bool{
			RoleOwner:    {ActionJobSubmit: true, ActionUserAdmin: true, ActionSettingsWrite: true, ActionAuditRead: true},
			RoleOperator: {ActionJobSubmit: true},
			RoleViewer:   {ActionAuditRead: true},
		},
	})
}

func TestAllowOwnerSubmit(t *testing.T) {
	g := testGuard()
	if err := g.Allow(context.Background(), Actor{UserID: "u1"}, "ws1", ActionJobSubmit); err != nil {
		t.Fatal(err)
	}
}

func TestAllowViewerCannotSubmit(t *testing.T) {
	g := testGuard()
	if err := g.Allow(context.Background(), Actor{UserID: "u2"}, "ws1", ActionJobSubmit); !errors.Is(err, ErrForbidden) && err == nil {
		t.Fatalf("want forbidden, got %v", err)
	}
}

func TestAllowDisabled(t *testing.T) {
	g := testGuard()
	if err := g.Allow(context.Background(), Actor{UserID: "off"}, "ws1", ActionJobSubmit); err == nil {
		t.Fatal("disabled user must be rejected")
	}
}

func TestAllowEmptyActor(t *testing.T) {
	g := testGuard()
	if err := g.Allow(context.Background(), Actor{}, "ws1", ActionJobSubmit); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("got %v", err)
	}
}

func TestAllowOwnerSettingsWrite(t *testing.T) {
	g := testGuard()
	if err := g.Allow(context.Background(), Actor{UserID: "u1"}, "ws1", ActionSettingsWrite); err != nil {
		t.Fatal(err)
	}
	if err := g.Allow(context.Background(), Actor{UserID: "u2"}, "ws1", ActionSettingsWrite); !errors.Is(err, ErrForbidden) {
		t.Fatalf("viewer must not write settings, got %v", err)
	}
}

func TestCanReadViewer(t *testing.T) {
	g := testGuard()
	if err := g.CanRead(context.Background(), Actor{UserID: "u2"}, "ws1"); err != nil {
		t.Fatal(err)
	}
}
