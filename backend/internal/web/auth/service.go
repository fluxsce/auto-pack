package auth

import (
	"errors"
	"time"

	"auto-pack/backend/internal/repository"

	"golang.org/x/crypto/bcrypt"
)

var (
	ErrInvalidCredentials = errors.New("invalid username or password")
	ErrUsernameTaken      = errors.New("username already exists")
	ErrWeakPassword       = errors.New("password must be at least 6 characters")
	ErrAccountDisabled    = errors.New("account is disabled")
)

// Service handles authentication business logic.
type Service struct {
	users  *repository.UserRepository
	tokens *TokenIssuer
}

func NewService(users *repository.UserRepository, tokens *TokenIssuer) *Service {
	return &Service{users: users, tokens: tokens}
}

type LoginResult struct {
	Token     string   `json:"token"`
	ExpiresAt string   `json:"expires_at"`
	User      UserView `json:"user"`
}

type UserView struct {
	RecID     string `json:"rec_id"`
	LoginName string `json:"login_name"`
}

func (s *Service) Login(loginName, password string) (*LoginResult, error) {
	u, err := s.users.FindByLoginName(loginName)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrInvalidCredentials
		}
		return nil, err
	}
	if u.IsEnabled != 1 {
		return nil, ErrAccountDisabled
	}
	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)); err != nil {
		return nil, ErrInvalidCredentials
	}

	token, expiresAt, err := s.tokens.Issue(u.RecID, u.LoginName)
	if err != nil {
		return nil, err
	}

	return &LoginResult{
		Token:     token,
		ExpiresAt: expiresAt.Format(time.RFC3339),
		User:      UserView{RecID: u.RecID, LoginName: u.LoginName},
	}, nil
}

func (s *Service) GetUser(userRecID string) (*UserView, error) {
	u, err := s.users.FindByRecID(userRecID)
	if err != nil {
		return nil, err
	}
	return &UserView{RecID: u.RecID, LoginName: u.LoginName}, nil
}

func (s *Service) ChangePassword(userRecID, oldPassword, newPassword string) error {
	if len(newPassword) < 6 {
		return ErrWeakPassword
	}
	u, err := s.users.FindByRecID(userRecID)
	if err != nil {
		return err
	}
	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(oldPassword)); err != nil {
		return ErrInvalidCredentials
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	return s.users.UpdatePassword(userRecID, string(hash))
}

func (s *Service) ChangeLoginName(userRecID, password, newLoginName string) error {
	if newLoginName == "" {
		return errors.New("username is required")
	}
	u, err := s.users.FindByRecID(userRecID)
	if err != nil {
		return err
	}
	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)); err != nil {
		return ErrInvalidCredentials
	}
	if u.LoginName == newLoginName {
		return nil
	}
	if _, err := s.users.FindByLoginName(newLoginName); err == nil {
		return ErrUsernameTaken
	} else if !errors.Is(err, repository.ErrNotFound) {
		return err
	}
	return s.users.UpdateLoginName(userRecID, newLoginName)
}
