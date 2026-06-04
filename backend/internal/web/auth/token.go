package auth

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const tokenTTL = 24 * time.Hour

type tokenClaims struct {
	UserRecID string `json:"uid"`
	LoginName string `json:"login_name"`
	jwt.RegisteredClaims
}

// TokenClaims is the validated JWT payload exposed to other packages.
type TokenClaims struct {
	UserRecID string
	LoginName string
}

// TokenIssuer issues and validates JWT access tokens.
type TokenIssuer struct {
	secret []byte
}

func NewTokenIssuer(secret string) *TokenIssuer {
	return &TokenIssuer{secret: []byte(secret)}
}

func (t *TokenIssuer) Issue(userRecID, loginName string) (string, time.Time, error) {
	expiresAt := time.Now().Add(tokenTTL)
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, tokenClaims{
		UserRecID: userRecID,
		LoginName: loginName,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	})
	signed, err := token.SignedString(t.secret)
	return signed, expiresAt, err
}

func (t *TokenIssuer) Parse(tokenString string) (*TokenClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &tokenClaims{}, func(token *jwt.Token) (any, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, errors.New("unexpected signing method")
		}
		return t.secret, nil
	})
	if err != nil {
		return nil, err
	}
	c, ok := token.Claims.(*tokenClaims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid token")
	}
	return &TokenClaims{UserRecID: c.UserRecID, LoginName: c.LoginName}, nil
}
