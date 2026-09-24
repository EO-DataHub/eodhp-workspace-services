package services

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	ws_manager "github.com/EO-DataHub/eodhp-workspace-manager/models"
	"github.com/EO-DataHub/eodhp-workspace-services/api/middleware"
	"github.com/EO-DataHub/eodhp-workspace-services/internal/authn"
	"github.com/EO-DataHub/eodhp-workspace-services/models"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockKeycloakClient is a mock implementation of KeycloakClientInterface

func TestCreateWorkspaceService(t *testing.T) {
	// Mock database, publisher, and Keycloak client
	mockDB := new(MockWorkspaceDB)
	mockPublisher := new(MockEventPublisher)
	mockKC := new(MockKeycloakClient)

	// Initialize the service with the mock dependencies
	svc := WorkspaceService{
		DB:        mockDB,
		Publisher: mockPublisher,
		KC:        mockKC,
	}

	// Mock claims for authentication
	mockClaims := authn.Claims{
		Username: "testuser",
	}

	// Valid workspace payload
	workspacePayload := ws_manager.WorkspaceSettings{
		Name:    "test-workspace",
		Account: uuid.New(),
	}

	expectedWorkspace := workspacePayload
	expectedWorkspace.Status = "creating"

	payloadBytes, _ := json.Marshal(workspacePayload)

	mockDB.On("CheckAccountIsVerified", workspacePayload.Account).Return(true, nil).Once()
	mockDB.On("CheckWorkspaceExists", workspacePayload.Name).Return(false, nil).Once()
	mockKC.On("CreateGroup", workspacePayload.Name).Return(http.StatusCreated, nil).Once()
	mockKC.On("GetGroup", workspacePayload.Name).Return(&models.Group{ID: "group-123"}, nil).Once()
	mockKC.On("AddMemberToGroup", mockClaims.Subject, "group-123").Return(nil).Once()
	mockDB.On("CreateWorkspace", mock.Anything).Return(&sql.Tx{}, nil).Once()
	mockDB.On("CommitTransaction", mock.Anything).Return(nil).Once()
	mockPublisher.On("Publish", mock.Anything).Return(nil).Once()

	// Create test request
	req := httptest.NewRequest(http.MethodPost, "/api/workspaces", bytes.NewReader(payloadBytes))
	req.Header.Set("Content-Type", "application/json")

	ctx := context.WithValue(req.Context(), middleware.ClaimsKey, mockClaims)
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()

	// Call the service method
	svc.CreateWorkspaceService(w, req)

	res := w.Result()
	defer res.Body.Close()

	assert.Equal(t, http.StatusCreated, res.StatusCode, "Expected HTTP status 201 Created")

	mockDB.AssertExpectations(t)
	mockPublisher.AssertExpectations(t)
	mockKC.AssertExpectations(t)

	req = httptest.NewRequest(http.MethodPost, "/api/workspaces", bytes.NewReader(payloadBytes))
	req.Header.Set("Content-Type", "application/json")

	w = httptest.NewRecorder()
	svc.CreateWorkspaceService(w, req)

	res = w.Result()
	defer res.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, res.StatusCode, "Expected HTTP status 401 Unauthorized")

	// Invalid JSON payload
	req = httptest.NewRequest(http.MethodPost, "/api/workspaces", bytes.NewReader([]byte("{invalid json}")))
	req.Header.Set("Content-Type", "application/json")

	ctx = context.WithValue(req.Context(), middleware.ClaimsKey, mockClaims)
	req = req.WithContext(ctx)

	w = httptest.NewRecorder()
	svc.CreateWorkspaceService(w, req)

	res = w.Result()
	defer res.Body.Close()

	assert.Equal(t, http.StatusBadRequest, res.StatusCode, "Expected HTTP status 400 Bad Request for invalid JSON")

	// Workspace name already exists
	mockDB.On("CheckAccountIsVerified", workspacePayload.Account).Return(true, nil).Once()
	mockDB.On("CheckWorkspaceExists", workspacePayload.Name).Return(true, nil).Once()

	req = httptest.NewRequest(http.MethodPost, "/api/workspaces", bytes.NewReader(payloadBytes))
	req.Header.Set("Content-Type", "application/json")

	ctx = context.WithValue(req.Context(), middleware.ClaimsKey, mockClaims)
	req = req.WithContext(ctx)

	w = httptest.NewRecorder()
	svc.CreateWorkspaceService(w, req)

	res = w.Result()
	defer res.Body.Close()

	assert.Equal(t, http.StatusConflict, res.StatusCode, "Expected HTTP status 409 Conflict for existing workspace")

	// Database error during account check
	mockDB.On("CheckAccountIsVerified", workspacePayload.Account).Return(false, fmt.Errorf("database error")).Once()

	req = httptest.NewRequest(http.MethodPost, "/api/workspaces", bytes.NewReader(payloadBytes))
	req.Header.Set("Content-Type", "application/json")

	ctx = context.WithValue(req.Context(), middleware.ClaimsKey, mockClaims)
	req = req.WithContext(ctx)

	w = httptest.NewRecorder()
	svc.CreateWorkspaceService(w, req)

	res = w.Result()
	defer res.Body.Close()

	assert.Equal(t, http.StatusInternalServerError, res.StatusCode, "Expected HTTP status 500 Internal Server Error for database error")
}

func TestGetWorkspacesService_Admin(t *testing.T) {
	mockDB := new(MockWorkspaceDB)
	mockKC := new(MockKeycloakClient)

	svc := WorkspaceService{
		DB: mockDB,
		KC: mockKC,
	}

	mockClaims := authn.Claims{
		Username: "testuser",
	}

	adminWorkspaces := []ws_manager.WorkspaceSettings{
		{Name: "owned-workspace"},
		{Name: "explicit-admin-workspace"},
	}

	mockDB.On("GetAdminWorkspaces", mockClaims.Username).Return(adminWorkspaces, nil).Once()

	req := httptest.NewRequest(http.MethodGet, "/api/workspaces?admin", nil)
	ctx := context.WithValue(req.Context(), middleware.ClaimsKey, mockClaims)
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	svc.GetWorkspacesService(w, req)

	res := w.Result()
	defer res.Body.Close()

	assert.Equal(t, http.StatusOK, res.StatusCode, "Expected HTTP status 200 OK")

	var result []ws_manager.WorkspaceSettings
	assert.NoError(t, json.NewDecoder(res.Body).Decode(&result))
	assert.Equal(t, adminWorkspaces, result)

	mockDB.AssertExpectations(t)
	mockKC.AssertExpectations(t)
}

func TestGetWorkspacesService_AdminDatabaseError(t *testing.T) {
	mockDB := new(MockWorkspaceDB)
	mockKC := new(MockKeycloakClient)

	svc := WorkspaceService{
		DB: mockDB,
		KC: mockKC,
	}

	mockClaims := authn.Claims{
		Username: "testuser",
	}

	mockDB.On("GetAdminWorkspaces", mockClaims.Username).Return([]ws_manager.WorkspaceSettings(nil), fmt.Errorf("database error")).Once()

	req := httptest.NewRequest(http.MethodGet, "/api/workspaces?admin", nil)
	ctx := context.WithValue(req.Context(), middleware.ClaimsKey, mockClaims)
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	svc.GetWorkspacesService(w, req)

	res := w.Result()
	defer res.Body.Close()

	assert.Equal(t, http.StatusInternalServerError, res.StatusCode, "Expected HTTP status 500 Internal Server Error for database error")

	mockDB.AssertExpectations(t)
	mockKC.AssertExpectations(t)
}

func deleteWorkspaceRequest(workspaceID string, claims authn.Claims) *http.Request {
	req := httptest.NewRequest(http.MethodDelete, "/workspaces/"+workspaceID, nil)
	req = mux.SetURLVars(req, map[string]string{"workspace-id": workspaceID})
	return req.WithContext(context.WithValue(req.Context(), middleware.ClaimsKey, claims))
}

// TestDeleteWorkspaceService_Owner confirms the account owner can delete their workspace.
func TestDeleteWorkspaceService_Owner(t *testing.T) {
	mockDB := new(MockWorkspaceDB)
	mockKC := new(MockKeycloakClient)
	mockPublisher := new(MockEventPublisher)
	svc := &WorkspaceService{DB: mockDB, KC: mockKC, Publisher: mockPublisher}

	claims := authn.Claims{Username: "owner-user"}
	claims.Subject = "owner-subject"

	mockKC.On("GetUserGroups", "owner-subject").Return([]string{"ws-owned"}, nil).Once()
	mockDB.On("IsUserAccountOwner", "owner-user", "ws-owned").Return(true, nil).Once()
	mockPublisher.On("Publish", mock.MatchedBy(func(ws ws_manager.WorkspaceSettings) bool {
		return ws.Name == "ws-owned" && ws.Status == "deleting"
	})).Return(nil).Once()

	rec := httptest.NewRecorder()
	svc.DeleteWorkspaceService(rec, deleteWorkspaceRequest("ws-owned", claims))

	assert.Equal(t, http.StatusNoContent, rec.Code)
	mockDB.AssertExpectations(t)
	mockKC.AssertExpectations(t)
	mockPublisher.AssertExpectations(t)
}

// TestDeleteWorkspaceService_NonMemberForbidden confirms a user cannot delete a workspace
// they have nothing to do with - previously any authenticated user could.
func TestDeleteWorkspaceService_NonMemberForbidden(t *testing.T) {
	mockDB := new(MockWorkspaceDB)
	mockKC := new(MockKeycloakClient)
	mockPublisher := new(MockEventPublisher)
	svc := &WorkspaceService{DB: mockDB, KC: mockKC, Publisher: mockPublisher}

	claims := authn.Claims{Username: "other-user"}
	claims.Subject = "other-subject"

	mockKC.On("GetUserGroups", "other-subject").Return([]string{"ws-mine"}, nil).Once()

	rec := httptest.NewRecorder()
	svc.DeleteWorkspaceService(rec, deleteWorkspaceRequest("ws-someone-elses", claims))

	assert.Equal(t, http.StatusForbidden, rec.Code)
	mockPublisher.AssertNotCalled(t, "Publish", mock.Anything)
}

// TestDeleteWorkspaceService_MemberForbidden confirms a plain member (neither the account
// owner nor a workspace admin) cannot delete the workspace.
func TestDeleteWorkspaceService_MemberForbidden(t *testing.T) {
	mockDB := new(MockWorkspaceDB)
	mockKC := new(MockKeycloakClient)
	mockPublisher := new(MockEventPublisher)
	svc := &WorkspaceService{DB: mockDB, KC: mockKC, Publisher: mockPublisher}

	claims := authn.Claims{Username: "member-user"}
	claims.Subject = "member-subject"

	mockKC.On("GetUserGroups", "member-subject").Return([]string{"ws-shared"}, nil).Once()
	mockDB.On("IsUserAccountOwner", "member-user", "ws-shared").Return(false, nil).Once()
	mockDB.On("IsUserWorkspaceAdmin", "member-user", "ws-shared").Return(false, nil).Once()

	rec := httptest.NewRecorder()
	svc.DeleteWorkspaceService(rec, deleteWorkspaceRequest("ws-shared", claims))

	assert.Equal(t, http.StatusForbidden, rec.Code)
	mockPublisher.AssertNotCalled(t, "Publish", mock.Anything)
}

// TestDeleteWorkspaceService_HubAdmin confirms a hub_admin can delete any workspace.
func TestDeleteWorkspaceService_HubAdmin(t *testing.T) {
	mockPublisher := new(MockEventPublisher)
	svc := &WorkspaceService{Publisher: mockPublisher}

	claims := authn.Claims{Username: "admin-user"}
	claims.RealmAccess.Roles = []string{"hub_admin"}

	mockPublisher.On("Publish", mock.Anything).Return(nil).Once()

	rec := httptest.NewRecorder()
	svc.DeleteWorkspaceService(rec, deleteWorkspaceRequest("ws-any", claims))

	assert.Equal(t, http.StatusNoContent, rec.Code)
	mockPublisher.AssertExpectations(t)
}
