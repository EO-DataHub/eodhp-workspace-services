package services

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	ws_manager "github.com/EO-DataHub/eodhp-workspace-manager/models"
	"github.com/EO-DataHub/eodhp-workspace-services/api/middleware"
	"github.com/EO-DataHub/eodhp-workspace-services/db"
	"github.com/EO-DataHub/eodhp-workspace-services/internal/appconfig"
	"github.com/EO-DataHub/eodhp-workspace-services/internal/authn"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func newWorkspaceCategoryRequest(workspaceID, body string, claims authn.Claims) *http.Request {
	req := httptest.NewRequest(http.MethodPut, "/workspaces/"+workspaceID+"/category", strings.NewReader(body))
	req = mux.SetURLVars(req, map[string]string{"workspace-id": workspaceID})
	ctx := context.WithValue(req.Context(), middleware.ClaimsKey, claims)
	return req.WithContext(ctx)
}

// categoryArg matches the category pointer passed to SetWorkspaceCategory by value.
func categoryArg(category string) interface{} {
	return mock.MatchedBy(func(c *string) bool { return c != nil && *c == category })
}

// nilCategoryArg matches a cleared category.
func nilCategoryArg() interface{} {
	return mock.MatchedBy(func(c *string) bool { return c == nil })
}

func newWorkspaceCategoryService(mockDB *MockWorkspaceDB, mockPublisher *MockEventPublisher, mockKC *MockKeycloakClient) *WorkspaceService {
	return &WorkspaceService{
		Config: &appconfig.Config{
			Workspaces: appconfig.WorkspacesConfig{Categories: []string{"standard", "academic", "commercial"}},
		},
		DB:        mockDB,
		Publisher: mockPublisher,
		KC:        mockKC,
	}
}

func TestSetWorkspaceCategoryServiceHubAdmin(t *testing.T) {
	t.Parallel()

	workspaceID := "ws-1"
	category := "academic"
	workspace := &ws_manager.WorkspaceSettings{Name: workspaceID, Status: "Available", Category: &category}

	mockDB := new(MockWorkspaceDB)
	tx := &sql.Tx{}
	mockDB.On("SetWorkspaceCategory", workspaceID, categoryArg(category)).Return(workspace, tx, nil)
	mockDB.On("CommitTransaction", tx).Return(nil).Once()

	// The republished settings must carry the new category, with a status workspace-manager accepts
	mockPublisher := new(MockEventPublisher)
	mockPublisher.On("Publish", mock.MatchedBy(func(ws ws_manager.WorkspaceSettings) bool {
		return ws.Name == workspaceID && ws.Status == "updating" && ws.Category != nil && *ws.Category == category
	})).Return(nil)

	svc := newWorkspaceCategoryService(mockDB, mockPublisher, new(MockKeycloakClient))

	rec := httptest.NewRecorder()
	svc.SetWorkspaceCategoryService(rec, newWorkspaceCategoryRequest(workspaceID, `{"category": "academic"}`, hubAdminClaims()))

	require.Equal(t, http.StatusNoContent, rec.Code)
	mockDB.AssertExpectations(t)
	mockPublisher.AssertExpectations(t)
}

// TestSetWorkspaceCategoryServiceForbiddenForOwner confirms an account owner can't set their own
// workspace's category, since a category is a discount.
func TestSetWorkspaceCategoryServiceForbiddenForOwner(t *testing.T) {
	t.Parallel()

	claims := authn.Claims{Username: "owner-user"}
	claims.Subject = "owner-subject"

	mockDB := new(MockWorkspaceDB)
	mockPublisher := new(MockEventPublisher)
	svc := newWorkspaceCategoryService(mockDB, mockPublisher, new(MockKeycloakClient))

	rec := httptest.NewRecorder()
	svc.SetWorkspaceCategoryService(rec, newWorkspaceCategoryRequest("ws-1", `{"category": "academic"}`, claims))

	require.Equal(t, http.StatusForbidden, rec.Code)
	mockDB.AssertNotCalled(t, "SetWorkspaceCategory", mock.Anything, mock.Anything)
	mockPublisher.AssertNotCalled(t, "Publish", mock.Anything)
}

func TestSetWorkspaceCategoryServiceWorkspaceScopedToken(t *testing.T) {
	t.Parallel()

	claims := hubAdminClaims()
	claims.Workspace = "ws-1"

	mockDB := new(MockWorkspaceDB)
	svc := newWorkspaceCategoryService(mockDB, new(MockEventPublisher), new(MockKeycloakClient))

	rec := httptest.NewRecorder()
	svc.SetWorkspaceCategoryService(rec, newWorkspaceCategoryRequest("ws-1", `{"category": "academic"}`, claims))

	require.Equal(t, http.StatusUnauthorized, rec.Code)
	mockDB.AssertNotCalled(t, "SetWorkspaceCategory", mock.Anything, mock.Anything)
}

func TestSetWorkspaceCategoryServiceInvalidCategory(t *testing.T) {
	t.Parallel()

	for _, body := range []string{`{"category": "premium"}`, `{"category": ""}`, `{}`, `{invalid json}`} {
		mockDB := new(MockWorkspaceDB)
		svc := newWorkspaceCategoryService(mockDB, new(MockEventPublisher), new(MockKeycloakClient))

		rec := httptest.NewRecorder()
		svc.SetWorkspaceCategoryService(rec, newWorkspaceCategoryRequest("ws-1", body, hubAdminClaims()))

		require.Equal(t, http.StatusBadRequest, rec.Code, body)
		mockDB.AssertNotCalled(t, "SetWorkspaceCategory", mock.Anything, mock.Anything)
	}
}

// TestSetWorkspaceCategoryServiceNoCategoriesConfigured confirms a missing categories config is
// reported as a server problem, not as the client sending a bad category.
func TestSetWorkspaceCategoryServiceNoCategoriesConfigured(t *testing.T) {
	t.Parallel()

	mockDB := new(MockWorkspaceDB)
	svc := newWorkspaceCategoryService(mockDB, new(MockEventPublisher), new(MockKeycloakClient))
	svc.Config.Workspaces.Categories = nil

	rec := httptest.NewRecorder()
	svc.SetWorkspaceCategoryService(rec, newWorkspaceCategoryRequest("ws-1", `{"category": "academic"}`, hubAdminClaims()))

	require.Equal(t, http.StatusServiceUnavailable, rec.Code)
	mockDB.AssertNotCalled(t, "SetWorkspaceCategory", mock.Anything, mock.Anything)
}

func TestSetWorkspaceCategoryServiceWorkspaceNotFound(t *testing.T) {
	t.Parallel()

	mockDB := new(MockWorkspaceDB)
	mockDB.On("SetWorkspaceCategory", "missing", categoryArg("academic")).Return(nil, nil, db.ErrWorkspaceNotFound)

	mockPublisher := new(MockEventPublisher)
	svc := newWorkspaceCategoryService(mockDB, mockPublisher, new(MockKeycloakClient))

	rec := httptest.NewRecorder()
	svc.SetWorkspaceCategoryService(rec, newWorkspaceCategoryRequest("missing", `{"category": "academic"}`, hubAdminClaims()))

	require.Equal(t, http.StatusNotFound, rec.Code)
	mockPublisher.AssertNotCalled(t, "Publish", mock.Anything)
}

func TestSetWorkspaceCategoryServicePublishFailure(t *testing.T) {
	t.Parallel()

	workspaceID := "ws-1"
	tx := &sql.Tx{}

	// A failed publish must roll back the category, so the DB and accounting-service still agree
	mockDB := new(MockWorkspaceDB)
	mockDB.On("SetWorkspaceCategory", workspaceID, categoryArg("commercial")).Return(&ws_manager.WorkspaceSettings{Name: workspaceID}, tx, nil)
	mockDB.On("RollbackTransaction", tx).Return(nil).Once()

	mockPublisher := new(MockEventPublisher)
	mockPublisher.On("Publish", mock.Anything).Return(errors.New("pulsar unavailable"))

	svc := newWorkspaceCategoryService(mockDB, mockPublisher, new(MockKeycloakClient))

	rec := httptest.NewRecorder()
	svc.SetWorkspaceCategoryService(rec, newWorkspaceCategoryRequest(workspaceID, `{"category": "commercial"}`, hubAdminClaims()))

	require.Equal(t, http.StatusInternalServerError, rec.Code)
	mockDB.AssertExpectations(t)
	mockDB.AssertNotCalled(t, "CommitTransaction", mock.Anything)
}

func TestSetWorkspaceCategoryServiceCommitFailure(t *testing.T) {
	t.Parallel()

	workspaceID := "ws-1"
	tx := &sql.Tx{}

	mockDB := new(MockWorkspaceDB)
	mockDB.On("SetWorkspaceCategory", workspaceID, categoryArg("commercial")).Return(&ws_manager.WorkspaceSettings{Name: workspaceID}, tx, nil)
	mockDB.On("CommitTransaction", tx).Return(errors.New("connection lost")).Once()

	mockPublisher := new(MockEventPublisher)
	mockPublisher.On("Publish", mock.Anything).Return(nil)

	svc := newWorkspaceCategoryService(mockDB, mockPublisher, new(MockKeycloakClient))

	rec := httptest.NewRecorder()
	svc.SetWorkspaceCategoryService(rec, newWorkspaceCategoryRequest(workspaceID, `{"category": "commercial"}`, hubAdminClaims()))

	require.Equal(t, http.StatusInternalServerError, rec.Code)
	mockDB.AssertExpectations(t)
}

func TestSetWorkspaceCategoryServiceClear(t *testing.T) {
	t.Parallel()

	workspaceID := "ws-1"
	tx := &sql.Tx{}

	mockDB := new(MockWorkspaceDB)
	mockDB.On("SetWorkspaceCategory", workspaceID, nilCategoryArg()).Return(&ws_manager.WorkspaceSettings{Name: workspaceID}, tx, nil)
	mockDB.On("CommitTransaction", tx).Return(nil).Once()

	// The republished settings carry no category, so the workspace is back on the default rate
	mockPublisher := new(MockEventPublisher)
	mockPublisher.On("Publish", mock.MatchedBy(func(ws ws_manager.WorkspaceSettings) bool {
		return ws.Name == workspaceID && ws.Status == "updating" && ws.Category == nil
	})).Return(nil)

	// Clearing needs no configured categories
	svc := newWorkspaceCategoryService(mockDB, mockPublisher, new(MockKeycloakClient))
	svc.Config.Workspaces.Categories = nil

	rec := httptest.NewRecorder()
	svc.SetWorkspaceCategoryService(rec, newWorkspaceCategoryRequest(workspaceID, `{"category": null}`, hubAdminClaims()))

	require.Equal(t, http.StatusNoContent, rec.Code)
	mockDB.AssertExpectations(t)
	mockPublisher.AssertExpectations(t)
}
