package authn

import (
	"errors"

	"github.com/golang-jwt/jwt"
)

var ErrInvalidJWT = errors.New("invalid jwt token")
var ErrInvalidClaims = errors.New("invalid claims")

type Claims struct {
	jwt.StandardClaims
	Username    string `json:"preferred_username"`
	RealmAccess struct {
		Roles []string `json:"roles"`
	} `json:"realm_access"`
	Workspace    string   `json:"workspace"`
	MemberGroups []string `json:"member_groups"`
}

func ParseClaims(token string) (Claims, error) {
	claims := Claims{}
	// Check if token is JWT by attempting to parse it
	if t, err := jwt.ParseWithClaims(token, &claims, nil); err != nil {
		// Ignore validation errors (no need to check signing of key)
		if _, ok := err.(*jwt.ValidationError); !ok {
			return claims, ErrInvalidJWT
		}

		// Check if token was decoded successfully
		if t == nil {
			// Return an error if the token was not decoded successfully
			return claims, ErrInvalidClaims
		}
	}
	return claims, nil
}
