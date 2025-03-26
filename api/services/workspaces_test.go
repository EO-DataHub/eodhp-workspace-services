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

	// Expected response workspace
	expectedWorkspace := workspacePayload
	expectedWorkspace.Status = "creating"

	// Marshal payload to JSON
	payloadBytes, _ := json.Marshal(workspacePayload)

	// Scenario 1: Successful workspace creation
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

	// Attach claims to request context
	ctx := context.WithValue(req.Context(), middleware.ClaimsKey, mockClaims)
	req = req.WithContext(ctx)

	// Create response recorder
	w := httptest.NewRecorder()

	// Call the CreateWorkspaceService method
	svc.CreateWorkspaceService(w, req)

	// Get response and assert HTTP 201 Created
	res := w.Result()
	defer res.Body.Close()

	assert.Equal(t, http.StatusCreated, res.StatusCode, "Expected HTTP status 201 Created")

	// Verify all expected method calls
	mockDB.AssertExpectations(t)
	mockPublisher.AssertExpectations(t)
	mockKC.AssertExpectations(t)

	// Scenario 2: Unauthorized request (missing claims)
	req = httptest.NewRequest(http.MethodPost, "/api/workspaces", bytes.NewReader(payloadBytes))
	req.Header.Set("Content-Type", "application/json")

	w = httptest.NewRecorder()
	svc.CreateWorkspaceService(w, req)

	res = w.Result()
	defer res.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, res.StatusCode, "Expected HTTP status 401 Unauthorized")

	// Scenario 3: Invalid JSON payload
	req = httptest.NewRequest(http.MethodPost, "/api/workspaces", bytes.NewReader([]byte("{invalid json}")))
	req.Header.Set("Content-Type", "application/json")

	ctx = context.WithValue(req.Context(), middleware.ClaimsKey, mockClaims)
	req = req.WithContext(ctx)

	w = httptest.NewRecorder()
	svc.CreateWorkspaceService(w, req)

	res = w.Result()
	defer res.Body.Close()

	assert.Equal(t, http.StatusBadRequest, res.StatusCode, "Expected HTTP status 400 Bad Request for invalid JSON")

	// Scenario 4: Workspace name already exists
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

	// Scenario 5: Database error during account check
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
