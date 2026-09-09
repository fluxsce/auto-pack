// Package token 签发与校验登录 JWT。无 HTTP。
package token

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

// Claims is the validated JWT payload.
type Claims struct {
	UserRecID string
	LoginName string
}

// Issuer issues and validates JWT access tokens.
type Issuer struct {
	secret []byte
}

// NewIssuer returns an Issuer.
func NewIssuer(secret string) *Issuer {
	return &Issuer{secret: []byte(secret)}
}

// Issue signs a token for the user.
func (t *Issuer) Issue(userRecID, loginName string) (string, time.Time, error) {
	expiresAt := time.Now().Add(tokenTTL)
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, tokenClaims{
		UserRecID: userRecID,
		LoginName: loginName,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	})
	signed, err := tok.SignedString(t.secret)
	return signed, expiresAt, err
}

// Parse validates a token string.
func (t *Issuer) Parse(tokenString string) (*Claims, error) {
	tok, err := jwt.ParseWithClaims(tokenString, &tokenClaims{}, func(tok *jwt.Token) (any, error) {
		if tok.Method != jwt.SigningMethodHS256 {
			return nil, errors.New("unexpected signing method")
		}
		return t.secret, nil
	})
	if err != nil {
		return nil, err
	}
	c, ok := tok.Claims.(*tokenClaims)
	if !ok || !tok.Valid {
		return nil, errors.New("invalid token")
	}
	return &Claims{UserRecID: c.UserRecID, LoginName: c.LoginName}, nil
}
