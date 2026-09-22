package authn

import (
	"crypto/rand"
	"encoding/base64"
	"errors"

	"github.com/golang-jwt/jwt/v5"
)

var ErrInvalidJWT = errors.New("invalid jwt token")
var ErrInvalidClaims = errors.New("invalid claims")

type Claims struct {
	jwt.RegisteredClaims
	Username    string `json:"preferred_username"`
	RealmAccess struct {
		Roles []string `json:"roles"`
	} `json:"realm_access"`
	Workspace string `json:"workspace"`
}

// GenerateToken creates a secure one-time token for account verification
func GenerateToken() (string, error) {
	b := make([]byte, 32) // 32-byte random token
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}
