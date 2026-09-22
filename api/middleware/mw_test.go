package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/EO-DataHub/eodhp-workspace-services/internal/authn"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
)

// testVerifier is a Verifier that never actually needs its key, since every test here
// sends a token that fails to parse before signature verification would run.
func testVerifier() *authn.Verifier {
	return authn.NewVerifierWithKeyFunc(func(*jwt.Token) (any, error) {
		return nil, jwt.ErrTokenUnverifiable
	})
}

func TestProxyAuthRequest_InvalidBearerToken_ClaimsNotPopulated(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims := r.Context().Value(ClaimsKey).(authn.Claims)
		// Test claims
		assert.Equal(t, "", claims.Subject)
		if claims.Subject == "" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	req, err := http.NewRequest("GET", "/test", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Add("Authorization", "Bearer invalid-token")

	mw := JWTMiddleware(testVerifier())(next)
	w := httptest.NewRecorder()
	mw.ServeHTTP(w, req)
}

func TestDenyWorkspaceScopedTokens_Middleware_WorkspaceScoped(t *testing.T) {
	// Define the next handler that will check if the claims are properly populated
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// This should not be reached if the token is workspace-scoped
		t.Fatal("Expected request to be blocked by DenyWorkspaceScopedTokens middleware")
	})

	// Create a mock request with a workspace-scoped token
	workspaceClaims := authn.Claims{
		Workspace: "user-scoped",
	}

	// Mock the context with the workspace-scoped claims
	ctx := context.WithValue(context.Background(), ClaimsKey, workspaceClaims)

	// Create a mock JWT token for the workspace-scoped claims
	token := "Bearer workspace-scoped-token"

	// Set up the request with the workspace-scoped token
	req, err := http.NewRequest("GET", "/accounts", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Add("Authorization", token)
	req = req.WithContext(ctx)

	mw := DenyWorkspaceScopedTokens(next)
	w := httptest.NewRecorder()
	mw.ServeHTTP(w, req)

	// Assert that the response status code is 401 Unauthorized for workspace-scoped tokens
	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401 Unauthorized but got %v", w.Code)
	}
}
