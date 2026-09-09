package access

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"auto-pack/server/internal/iam"
	"auto-pack/server/internal/store"
)

// Directory implements iam.Directory against AP_* tables.
type Directory struct {
	st *store.Store
}

// NewDirectory returns a Directory.
func NewDirectory(st *store.Store) *Directory {
	return &Directory{st: st}
}

// FindMember loads one workspace membership.
func (d *Directory) FindMember(ctx context.Context, workspaceID, userID string) (iam.Member, error) {
	q := d.st.Rebind(`SELECT workspace_rec_id, user_rec_id, member_role FROM AP_WORKSPACE_MEMBER
		WHERE workspace_rec_id = ? AND user_rec_id = ?`)
	var m iam.Member
	err := d.st.SQL().QueryRowContext(ctx, q, workspaceID, userID).Scan(&m.WorkspaceID, &m.UserID, &m.Role)
	if errors.Is(err, sql.ErrNoRows) {
		return iam.Member{}, err
	}
	return m, err
}

// RoleAllows reports whether role_code has action_code.
func (d *Directory) RoleAllows(ctx context.Context, role string, action iam.Action) (bool, error) {
	q := d.st.Rebind(`SELECT COUNT(1) FROM AP_ROLE_ACTION WHERE role_code = ? AND action_code = ?`)
	var n int
	if err := d.st.SQL().QueryRowContext(ctx, q, role, string(action)).Scan(&n); err != nil {
		return false, err
	}
	return n > 0, nil
}

// IsEnabledUser reports whether the user exists and is enabled.
func (d *Directory) IsEnabledUser(ctx context.Context, userID string) (bool, error) {
	q := d.st.Rebind(`SELECT is_enabled FROM AP_USER WHERE rec_id = ?`)
	var flag int
	err := d.st.SQL().QueryRowContext(ctx, q, userID).Scan(&flag)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return flag == 1, nil
}

// ListActions returns granted actions for a role.
func (d *Directory) ListActions(ctx context.Context, role string) ([]string, error) {
	q := d.st.Rebind(`SELECT action_code FROM AP_ROLE_ACTION WHERE role_code = ? ORDER BY action_code`)
	rows, err := d.st.SQL().QueryContext(ctx, q, role)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var a string
		if err := rows.Scan(&a); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// ReplaceRoleActions replaces grants for one role (user.admin only).
func (d *Directory) ReplaceRoleActions(ctx context.Context, role string, actions []string) error {
	role = strings.TrimSpace(role)
	if role == "" {
		return errors.New("role is required")
	}
	return d.st.Tx(ctx, func(tx *store.Store) error {
		del := tx.Rebind(`DELETE FROM AP_ROLE_ACTION WHERE role_code = ?`)
		if _, err := tx.SQL().ExecContext(ctx, del, role); err != nil {
			return err
		}
		ins := tx.Rebind(`INSERT INTO AP_ROLE_ACTION (rec_id, role_code, action_code) VALUES (?, ?, ?)`)
		for _, a := range actions {
			a = strings.TrimSpace(a)
			if a == "" {
				continue
			}
			id := NewID()
			if _, err := tx.SQL().ExecContext(ctx, ins, id, role, a); err != nil {
				return err
			}
		}
		return nil
	})
}
