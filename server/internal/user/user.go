package user

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"auto-pack/server/internal/access"
	"auto-pack/server/internal/iam"
	"auto-pack/server/internal/store"

	"golang.org/x/crypto/bcrypt"
)

const (
	defaultLogin    = "admin"
	defaultPassword = "123456"
)

var (
	// ErrInvalidCredentials 用户名或密码不对。
	ErrInvalidCredentials = errors.New("invalid username or password")
	// ErrAccountDisabled 账号已停用。
	ErrAccountDisabled = errors.New("account disabled")
)

// User is a login account.
type User struct {
	RecID        string `json:"rec_id"`
	LoginName    string `json:"login_name"`
	DisplayName  string `json:"display_name"`
	PasswordHash string `json:"-"`
	IsEnabled    int    `json:"is_enabled"`
	Remark       string `json:"remark,omitempty"`
}

// Service handles users and the role-action matrix. Write paths call iam.Allow.
type Service struct {
	st    *store.Store
	guard *iam.Guard
	dir   *access.Directory
}

// NewService returns a user Service.
func NewService(st *store.Store, guard *iam.Guard, dir *access.Directory) *Service {
	return &Service{st: st, guard: guard, dir: dir}
}

// SeedDefaultAdmin inserts admin/123456 when AP_USER is empty.
func SeedDefaultAdmin(ctx context.Context, st *store.Store) error {
	q := st.Rebind(`SELECT COUNT(1) FROM AP_USER`)
	var n int
	if err := st.SQL().QueryRowContext(ctx, q).Scan(&n); err != nil {
		return err
	}
	if n > 0 {
		return nil
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(defaultPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	ins := st.Rebind(`INSERT INTO AP_USER (rec_id, login_name, display_name, password_hash, is_enabled, remark)
		VALUES (?, ?, ?, ?, 1, ?)`)
	_, err = st.SQL().ExecContext(ctx, ins, access.DefaultAdminRecID, defaultLogin, "管理员", string(hash), "初始化默认账号")
	return err
}

// Authenticate 校验登录名与密码，供登录模块发 JWT。
func (s *Service) Authenticate(ctx context.Context, loginName, password string) (User, error) {
	u, err := s.FindByLoginName(ctx, loginName)
	if err != nil || bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)) != nil {
		return User{}, ErrInvalidCredentials
	}
	if u.IsEnabled != 1 {
		return User{}, ErrAccountDisabled
	}
	return u, nil
}

// ListRoleActions 读取角色允许的动作（user.admin）。
func (s *Service) ListRoleActions(ctx context.Context, actor iam.Actor, workspaceID, role string) ([]string, error) {
	if err := s.guard.Allow(ctx, actor, workspaceID, iam.ActionUserAdmin); err != nil {
		return nil, err
	}
	return s.dir.ListActions(ctx, role)
}

// ReplaceRoleActions 覆盖角色动作矩阵（user.admin）。
func (s *Service) ReplaceRoleActions(ctx context.Context, actor iam.Actor, workspaceID, role string, actions []string) error {
	if err := s.guard.Allow(ctx, actor, workspaceID, iam.ActionUserAdmin); err != nil {
		return err
	}
	return s.dir.ReplaceRoleActions(ctx, role, actions)
}

// FindByLoginName loads a user by login name.
func (s *Service) FindByLoginName(ctx context.Context, loginName string) (User, error) {
	q := s.st.Rebind(`SELECT rec_id, login_name, display_name, password_hash, is_enabled, COALESCE(remark,'')
		FROM AP_USER WHERE login_name = ?`)
	var u User
	err := s.st.SQL().QueryRowContext(ctx, q, strings.TrimSpace(loginName)).Scan(
		&u.RecID, &u.LoginName, &u.DisplayName, &u.PasswordHash, &u.IsEnabled, &u.Remark)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, err
	}
	return u, err
}

// FindByID loads a user by rec_id.
func (s *Service) FindByID(ctx context.Context, id string) (User, error) {
	q := s.st.Rebind(`SELECT rec_id, login_name, display_name, password_hash, is_enabled, COALESCE(remark,'')
		FROM AP_USER WHERE rec_id = ?`)
	var u User
	err := s.st.SQL().QueryRowContext(ctx, q, id).Scan(
		&u.RecID, &u.LoginName, &u.DisplayName, &u.PasswordHash, &u.IsEnabled, &u.Remark)
	return u, err
}

// List returns all users (user.admin).
func (s *Service) List(ctx context.Context, actor iam.Actor, workspaceID string) ([]User, error) {
	if err := s.guard.Allow(ctx, actor, workspaceID, iam.ActionUserAdmin); err != nil {
		return nil, err
	}
	q := `SELECT rec_id, login_name, display_name, password_hash, is_enabled, COALESCE(remark,'') FROM AP_USER ORDER BY login_name`
	rows, err := s.st.SQL().QueryContext(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []User
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.RecID, &u.LoginName, &u.DisplayName, &u.PasswordHash, &u.IsEnabled, &u.Remark); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

// Create adds a user (user.admin).
func (s *Service) Create(ctx context.Context, actor iam.Actor, workspaceID string, login, display, password string, enabled bool) (User, error) {
	if err := s.guard.Allow(ctx, actor, workspaceID, iam.ActionUserAdmin); err != nil {
		return User{}, err
	}
	login = strings.TrimSpace(login)
	if login == "" || len(password) < 6 {
		return User{}, errors.New("login and password (>=6) required")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return User{}, err
	}
	flag := 1
	if !enabled {
		flag = 0
	}
	if display == "" {
		display = login
	}
	id := access.NewID()
	ins := s.st.Rebind(`INSERT INTO AP_USER (rec_id, login_name, display_name, password_hash, is_enabled)
		VALUES (?, ?, ?, ?, ?)`)
	if _, err := s.st.SQL().ExecContext(ctx, ins, id, login, display, string(hash), flag); err != nil {
		return User{}, err
	}
	return s.FindByID(ctx, id)
}

// SetEnabled toggles a user (user.admin).
func (s *Service) SetEnabled(ctx context.Context, actor iam.Actor, workspaceID, userID string, enabled bool) error {
	if err := s.guard.Allow(ctx, actor, workspaceID, iam.ActionUserAdmin); err != nil {
		return err
	}
	flag := 1
	if !enabled {
		flag = 0
	}
	q := s.st.Rebind(`UPDATE AP_USER SET is_enabled = ?, updated_at = ? WHERE rec_id = ?`)
	_, err := s.st.SQL().ExecContext(ctx, q, flag, nowSQL(), userID)
	return err
}

func nowSQL() string {
	return time.Now().UTC().Format("2006-01-02 15:04:05")
}
