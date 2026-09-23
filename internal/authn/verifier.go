package authn

import (
	"fmt"

	"github.com/MicahParks/keyfunc/v3"
	"github.com/golang-jwt/jwt/v5"
)

// allowedAudiences are the Keycloak client IDs platform tokens are issued for (the audience
// mappers on the eodh and eodh-workspaces clients, eodhp-argocd-deployment
// apps/keycloak/base/realms.yaml). This list is duplicated across the platform's services,
// so change them together.
var allowedAudiences = []string{"eodh", "eodh-workspaces"}

// Verifier checks a JWT's signature against Keycloak's published key before trusting any
// claim in it, rather than assuming that has already been done further upstream. A
// gateway sitting in front of this service's public routes does not cover traffic that
// reaches it directly from elsewhere on the cluster network.
type Verifier struct {
	keyFunc jwt.Keyfunc
}

// NewVerifier builds a Verifier backed by the JWKS at certsURL, e.g.
// https://<platform-domain>/keycloak/realms/eodhp/protocol/openid-connect/certs. It
// fetches and caches the key set, refreshing it in the background for the life of the
// process, so verifying a token does not cost a round trip to Keycloak each time.
func NewVerifier(certsURL string) (*Verifier, error) {
	kf, err := keyfunc.NewDefault([]string{certsURL})
	if err != nil {
		return nil, fmt.Errorf("building JWKS keyfunc for %q: %w", certsURL, err)
	}

	return &Verifier{keyFunc: kf.Keyfunc}, nil
}

// NewVerifierWithKeyFunc builds a Verifier from an already-constructed jwt.Keyfunc,
// letting tests (and anything else that already has one) supply a fixed key rather than
// talking to a real JWKS endpoint.
func NewVerifierWithKeyFunc(keyFunc jwt.Keyfunc) *Verifier {
	return &Verifier{keyFunc: keyFunc}
}

// ParseClaims verifies tokenStr's signature and audience against Keycloak, then returns
// its claims. Every authorisation decision built on these claims (workspace ownership,
// realm roles) depends on this actually being checked here.
func (v *Verifier) ParseClaims(tokenStr string) (Claims, error) {
	claims := Claims{}

	token, err := jwt.ParseWithClaims(tokenStr, &claims, v.keyFunc, jwt.WithValidMethods([]string{"RS256"}))
	if err != nil || !token.Valid {
		return Claims{}, ErrInvalidJWT
	}

	if !hasAllowedAudience(claims.Audience) {
		return Claims{}, ErrInvalidClaims
	}

	return claims, nil
}

func hasAllowedAudience(aud jwt.ClaimStrings) bool {
	for _, a := range aud {
		for _, allowed := range allowedAudiences {
			if a == allowed {
				return true
			}
		}
	}

	return false
}
