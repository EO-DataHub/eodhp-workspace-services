package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/EO-DataHub/eodhp-workspace-services/internal/authn"
	"github.com/stretchr/testify/assert"
)

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

	mw := JWTMiddleware(next)
	w := httptest.NewRecorder()
	mw.ServeHTTP(w, req)
}
