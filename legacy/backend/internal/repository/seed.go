package repository

import (
	"database/sql"
	"errors"

	"golang.org/x/crypto/bcrypt"
)

const (
	defaultLoginName  = "admin"
	defaultPassword   = "123456"
	DefaultAdminRecID = "00000000-0000-4000-8000-000000000001"
	defaultAdminRecID = DefaultAdminRecID
)

// Seed inserts default data when tables are empty.
func Seed(db *sql.DB) error {
	if err := seedDefaultUser(db); err != nil {
		return err
	}
	return seedDefaultWorkspace(db)
}

func seedDefaultUser(db *sql.DB) error {
	repo := NewUser(db)
	count, err := repo.Count()
	if err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(defaultPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	return repo.Insert(&User{
		RecID:        defaultAdminRecID,
		LoginName:    defaultLoginName,
		PasswordHash: string(hash),
		IsEnabled:    1,
		Remark:       "初始化默认账号",
	})
}

// DefaultAdminCredentials exposes initial credentials for documentation only.
func DefaultAdminCredentials() (loginName, password string) {
	return defaultLoginName, defaultPassword
}

func seedDefaultWorkspace(db *sql.DB) error {
	repo := NewWorkspace(db)
	_, err := repo.FindByRecID(DefaultWorkspaceRecID)
	if err == nil {
		return nil
	}
	if !errors.Is(err, ErrWorkspaceNotFound) {
		return err
	}
	return repo.Insert(&PackWorkspace{
		RecID:         DefaultWorkspaceRecID,
		WorkspaceCode: "default",
		WorkspaceName: "默认工作空间",
		RootDir:       "",
		IsDefault:     1,
		IsEnabled:     1,
		SortNum:       0,
		Remark:        "系统初始化",
	})
}

// EnsureDefaultUserExists re-seeds default user if missing.
func EnsureDefaultUserExists(db *sql.DB) error {
	repo := NewUser(db)
	_, err := repo.FindByLoginName(defaultLoginName)
	if err == nil {
		return nil
	}
	if err != ErrNotFound {
		return err
	}
	return seedDefaultUser(db)
}
