package repository

import (
	"database/sql"
	"errors"
	"strings"
	"time"
)

// UserRepository provides AP_USER_AUTH persistence.
type UserRepository struct {
	db *sql.DB
}

func NewUser(db *sql.DB) *UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) FindByLoginName(loginName string) (*User, error) {
	row := r.db.QueryRow(`
		SELECT rec_id, login_name, password_hash, is_enabled, remark, created_at, updated_at
		FROM AP_USER_AUTH WHERE login_name = ?`, loginName)
	return scanUser(row)
}

func (r *UserRepository) FindByRecID(recID string) (*User, error) {
	row := r.db.QueryRow(`
		SELECT rec_id, login_name, password_hash, is_enabled, remark, created_at, updated_at
		FROM AP_USER_AUTH WHERE rec_id = ?`, recID)
	return scanUser(row)
}

func (r *UserRepository) UpdateLoginName(recID, loginName string) error {
	_, err := r.db.Exec(
		`UPDATE AP_USER_AUTH SET login_name = ?, updated_at = ? WHERE rec_id = ?`,
		loginName, nowTS(), recID,
	)
	return mapUserUniqueErr(err)
}

func (r *UserRepository) UpdatePassword(recID, passwordHash string) error {
	_, err := r.db.Exec(
		`UPDATE AP_USER_AUTH SET password_hash = ?, updated_at = ? WHERE rec_id = ?`,
		passwordHash, time.Now().UTC().Format("2006-01-02 15:04:05"), recID,
	)
	return err
}

func (r *UserRepository) Count() (int, error) {
	var n int
	err := r.db.QueryRow(`SELECT COUNT(*) FROM AP_USER_AUTH`).Scan(&n)
	return n, err
}

func (r *UserRepository) Insert(u *User) error {
	_, err := r.db.Exec(`
		INSERT INTO AP_USER_AUTH(rec_id, login_name, password_hash, is_enabled, remark)
		VALUES(?, ?, ?, ?, ?)`,
		u.RecID, u.LoginName, u.PasswordHash, u.IsEnabled, u.Remark,
	)
	return mapUserUniqueErr(err)
}

func (r *UserRepository) List() ([]User, error) {
	rows, err := r.db.Query(`
		SELECT rec_id, login_name, password_hash, is_enabled, remark, created_at, updated_at
		FROM AP_USER_AUTH
		ORDER BY created_at ASC, login_name ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list := make([]User, 0)
	for rows.Next() {
		item, err := scanUserRow(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, *item)
	}
	return list, rows.Err()
}

func (r *UserRepository) UpdateProfile(recID, loginName string, isEnabled int, remark string) error {
	res, err := r.db.Exec(`
		UPDATE AP_USER_AUTH SET login_name = ?, is_enabled = ?, remark = ?, updated_at = ?
		WHERE rec_id = ?`,
		loginName, isEnabled, remark, nowTS(), recID,
	)
	if err != nil {
		return mapUserUniqueErr(err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *UserRepository) Delete(recID string) error {
	res, err := r.db.Exec(`DELETE FROM AP_USER_AUTH WHERE rec_id = ?`, recID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *UserRepository) CountEnabled() (int, error) {
	var n int
	err := r.db.QueryRow(`SELECT COUNT(*) FROM AP_USER_AUTH WHERE is_enabled = 1`).Scan(&n)
	return n, err
}

func mapUserUniqueErr(err error) error {
	if err == nil {
		return nil
	}
	lower := strings.ToLower(err.Error())
	if strings.Contains(lower, "login_name") || strings.Contains(lower, "ix_ap_user_login") || strings.Contains(lower, "unique") {
		return ErrUserLoginNameExists
	}
	return err
}

type userScanner interface {
	Scan(dest ...any) error
}

func scanUserRow(sc userScanner) (*User, error) {
	var u User
	var created, updated string
	err := sc.Scan(&u.RecID, &u.LoginName, &u.PasswordHash, &u.IsEnabled, &u.Remark, &created, &updated)
	if err != nil {
		return nil, err
	}
	u.CreatedAt = parseTime(created)
	u.UpdatedAt = parseTime(updated)
	return &u, nil
}

func scanUser(row *sql.Row) (*User, error) {
	u, err := scanUserRow(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return u, nil
}
