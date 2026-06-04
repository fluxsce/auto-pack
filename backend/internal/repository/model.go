package repository

import (
	"errors"
	"time"
)

var ErrNotFound = errors.New("user not found")

var (
	ErrUserLoginNameExists  = errors.New("login name already exists")
	ErrUserCannotDeleteSelf = errors.New("cannot delete your own account")
	ErrUserLastEnabled      = errors.New("cannot disable or delete the last enabled user")
)

// User represents a row in AP_USER_AUTH.
type User struct {
	RecID        string    `json:"rec_id"`
	LoginName    string    `json:"login_name"`
	PasswordHash string    `json:"-"`
	IsEnabled    int       `json:"is_enabled"`
	Remark       string    `json:"remark"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}
