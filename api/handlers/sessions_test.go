package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/EO-DataHub/eodhp-workspace-services/api/middleware"
	services "github.com/EO-DataHub/eodhp-workspace-services/api/services"
	"github.com/EO-DataHub/eodhp-workspace-services/internal/authn"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
)

type keycloakMock struct {
	response *services.TokenResponse
	err      error
}

func (k keycloakMock) ExchangeToken(token, scope string) (
	*services.TokenResponse, error) {

	if k.err != nil {
		return nil, k.err
	}

	return k.response, nil
}

func TestCreateWorkspaceSession_Success(t *testing.T) {
	kc := &keycloakMock{
		response: &services.TokenResponse{
			Access:           "access-token",
			ExpiresIn:        3600,
			Refresh:          "refresh-token",
			RefreshExpiresIn: 7200,
			Scope:            "workspace:test",
		},
	}

	req := httptest.NewRequest(http.MethodPost, "/workspaces/{workspace-id}/{user-id}/sessions", nil)
	req = mux.SetURLVars(req, map[string]string{"workspace-id": "test", "user-id": "me"})
	ctx := context.WithValue(req.Context(), middleware.TokenKey, "valid-token")
	ctx = context.WithValue(ctx, middleware.ClaimsKey, authn.Claims{Username: "user"})

	w := httptest.NewRecorder()

	handler := CreateWorkspaceSession(kc)
	handler.ServeHTTP(w, req.WithContext(ctx))

	assert.Equal(t, http.StatusOK, w.Code)

	var response AuthSessionResponse
	err := json.NewDecoder(w.Body).Decode(&response)
	assert.NoError(t, err)
	assert.Equal(t, "access-token", response.Access)
	assert.Equal(t, "refresh-token", response.Refresh)
	assert.Equal(t, "workspace:test", response.Scope)
}

func TestCreateWorkspaceSession_InvalidToken(t *testing.T) {
	kc := &keycloakMock{
		err: &services.HTTPError{Status: http.StatusUnauthorized, Message: "Invalid token"},
	}

	req := httptest.NewRequest(http.MethodPost, "/workspaces/{workspace-id}/{user-id}/sessions", nil)
	req = mux.SetURLVars(req, map[string]string{"workspace-id": "test", "user-id": "me"})
	ctx := context.WithValue(req.Context(), middleware.TokenKey, "invalid-token")
	ctx = context.WithValue(ctx, middleware.ClaimsKey, authn.Claims{Username: "user"})

	w := httptest.NewRecorder()

	handler := CreateWorkspaceSession(kc)
	handler.ServeHTTP(w, req.WithContext(ctx))

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Contains(t, w.Body.String(), "Invalid token")
}

func TestCreateWorkspaceSession_ExchangeTokenError(t *testing.T) {
	kc := &keycloakMock{
		err: &services.HTTPError{Status: http.StatusForbidden, Message: "Forbidden"},
	}

	req := httptest.NewRequest(http.MethodPost, "/workspaces/{workspace-id}/{user-id}/sessions", nil)
	req = mux.SetURLVars(req, map[string]string{"workspace-id": "test", "user-id": "me"})
	ctx := context.WithValue(req.Context(), middleware.TokenKey, "unauthorized-token")
	ctx = context.WithValue(ctx, middleware.ClaimsKey, authn.Claims{Username: "user"})

	rec := httptest.NewRecorder()

	handler := CreateWorkspaceSession(kc)
	handler.ServeHTTP(rec, req.WithContext(ctx))

	assert.Equal(t, http.StatusForbidden, rec.Code)
	assert.Contains(t, rec.Body.String(), "Forbidden")
}

func TestCreateWorkspaceSession_WrongUserError(t *testing.T) {
	kc := &keycloakMock{
		err: &services.HTTPError{Status: http.StatusForbidden, Message: "Forbidden"},
	}

	req := httptest.NewRequest(http.MethodPost, "/workspaces/{workspace-id}/{user-id}/sessions", nil)
	req = mux.SetURLVars(req, map[string]string{"workspace-id": "test", "user-id": "another-user"})
	ctx := context.WithValue(req.Context(), middleware.TokenKey, "valid-token")
	ctx = context.WithValue(ctx, middleware.ClaimsKey, authn.Claims{Username: "user"})

	rec := httptest.NewRecorder()

	handler := CreateWorkspaceSession(kc)
	handler.ServeHTTP(rec, req.WithContext(ctx))

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}
