package services

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	ws_manager "github.com/EO-DataHub/eodhp-workspace-manager/models"
	"github.com/EO-DataHub/eodhp-workspace-services/api/middleware"
	"github.com/EO-DataHub/eodhp-workspace-services/internal/authn"
	ws_services "github.com/EO-DataHub/eodhp-workspace-services/models"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/require"
)

func newWorkspaceAdminRequest(method, workspaceID, username string, claims authn.Claims) *http.Request {
	url := "/workspaces/" + workspaceID + "/admins"
	vars := map[string]string{"workspace-id": workspaceID}
	if username != "" {
		url += "/" + username
		vars["username"] = username
	}

	req := httptest.NewRequest(method, url, nil)
	req = mux.SetURLVars(req, vars)
	ctx := context.WithValue(req.Context(), middleware.ClaimsKey, claims)
	return req.WithContext(ctx)
}

func TestGetWorkspaceAdminsServiceSuccess(t *testing.T) {
	t.Parallel()

	claims := authn.Claims{Username: "member-user"}
	claims.Subject = "member-subject"
	workspaceID := "ws-1"

	mockKC := new(MockKeycloakClient)
	mockKC.On("GetUserGroups", "member-subject").Return([]string{workspaceID}, nil)

	mockDB := new(MockWorkspaceDB)
	mockDB.On("GetWorkspaceAdmins", workspaceID).Return([]string{"admin-a", "admin-b"}, nil)

	svc := &WorkspaceService{DB: mockDB, KC: mockKC}

	rec := httptest.NewRecorder()
	svc.GetWorkspaceAdminsService(rec, newWorkspaceAdminRequest(http.MethodGet, workspaceID, "", claims))

	require.Equal(t, http.StatusOK, rec.Code)
	mockDB.AssertExpectations(t)
	mockKC.AssertExpectations(t)
}

func TestGetWorkspaceAdminsServiceForbiddenForNonMember(t *testing.T) {
	t.Parallel()

	claims := authn.Claims{Username: "outsider"}
	claims.Subject = "outsider-subject"
	workspaceID := "ws-1"

	mockKC := new(MockKeycloakClient)
	mockKC.On("GetUserGroups", "outsider-subject").Return([]string{"ws-other"}, nil)

	mockDB := new(MockWorkspaceDB)

	svc := &WorkspaceService{DB: mockDB, KC: mockKC}

	rec := httptest.NewRecorder()
	svc.GetWorkspaceAdminsService(rec, newWorkspaceAdminRequest(http.MethodGet, workspaceID, "", claims))

	require.Equal(t, http.StatusForbidden, rec.Code)
	mockDB.AssertExpectations(t)
	mockKC.AssertExpectations(t)
}

func TestAddWorkspaceAdminServiceSuccess(t *testing.T) {
	t.Parallel()

	adminClaims := authn.Claims{Username: "admin-user"}
	adminClaims.Subject = "admin-subject"
	workspaceID := "ws-1"
	targetUsername := "target-user"

	workspace := &ws_manager.WorkspaceSettings{Name: workspaceID}
	group := &ws_services.Group{ID: "group-1", Name: workspaceID}
	targetUser := &ws_services.User{ID: "target-id", Username: targetUsername}

	mockKC := new(MockKeycloakClient)
	mockKC.On("GetUserGroups", "admin-subject").Return([]string{workspaceID}, nil)
	mockKC.On("GetGroup", workspaceID).Return(group, nil)
	mockKC.On("GetUser", targetUsername).Return(targetUser, nil)
	mockKC.On("GetGroupMembers", "group-1").Return([]ws_services.User{*targetUser}, nil)

	mockDB := new(MockWorkspaceDB)
	mockDB.On("IsUserAccountOwner", "admin-user", workspaceID).Return(false, nil)
	mockDB.On("IsUserWorkspaceAdmin", "admin-user", workspaceID).Return(true, nil)
	mockDB.On("GetWorkspace", workspaceID).Return(workspace, nil)
	mockDB.On("AddWorkspaceAdmin", targetUsername, workspaceID, "admin-user").Return(nil)

	svc := &WorkspaceService{DB: mockDB, KC: mockKC}

	rec := httptest.NewRecorder()
	svc.AddWorkspaceAdminService(rec, newWorkspaceAdminRequest(http.MethodPut, workspaceID, targetUsername, adminClaims))

	require.Equal(t, http.StatusNoContent, rec.Code)
	mockDB.AssertExpectations(t)
	mockKC.AssertExpectations(t)
}

func TestAddWorkspaceAdminServiceForbiddenForNonAdmin(t *testing.T) {
	t.Parallel()

	memberClaims := authn.Claims{Username: "member-user"}
	memberClaims.Subject = "member-subject"
	workspaceID := "ws-1"
	targetUsername := "target-user"

	mockKC := new(MockKeycloakClient)
	mockKC.On("GetUserGroups", "member-subject").Return([]string{workspaceID}, nil)

	mockDB := new(MockWorkspaceDB)
	mockDB.On("IsUserAccountOwner", "member-user", workspaceID).Return(false, nil)
	mockDB.On("IsUserWorkspaceAdmin", "member-user", workspaceID).Return(false, nil)

	svc := &WorkspaceService{DB: mockDB, KC: mockKC}

	rec := httptest.NewRecorder()
	svc.AddWorkspaceAdminService(rec, newWorkspaceAdminRequest(http.MethodPut, workspaceID, targetUsername, memberClaims))

	require.Equal(t, http.StatusForbidden, rec.Code)
	mockDB.AssertExpectations(t)
	mockKC.AssertExpectations(t)
}

func TestAddWorkspaceAdminServiceRequiresExistingMembership(t *testing.T) {
	t.Parallel()

	adminClaims := authn.Claims{Username: "admin-user"}
	adminClaims.Subject = "admin-subject"
	workspaceID := "ws-1"
	targetUsername := "non-member"

	workspace := &ws_manager.WorkspaceSettings{Name: workspaceID}
	group := &ws_services.Group{ID: "group-1", Name: workspaceID}
	targetUser := &ws_services.User{ID: "target-id", Username: targetUsername}

	mockKC := new(MockKeycloakClient)
	mockKC.On("GetUserGroups", "admin-subject").Return([]string{workspaceID}, nil)
	mockKC.On("GetGroup", workspaceID).Return(group, nil)
	mockKC.On("GetUser", targetUsername).Return(targetUser, nil)
	mockKC.On("GetGroupMembers", "group-1").Return([]ws_services.User{}, nil)

	mockDB := new(MockWorkspaceDB)
	mockDB.On("IsUserAccountOwner", "admin-user", workspaceID).Return(true, nil)
	mockDB.On("GetWorkspace", workspaceID).Return(workspace, nil)

	svc := &WorkspaceService{DB: mockDB, KC: mockKC}

	rec := httptest.NewRecorder()
	svc.AddWorkspaceAdminService(rec, newWorkspaceAdminRequest(http.MethodPut, workspaceID, targetUsername, adminClaims))

	require.Equal(t, http.StatusBadRequest, rec.Code)
	mockDB.AssertExpectations(t)
	mockDB.AssertNotCalled(t, "AddWorkspaceAdmin")
	mockKC.AssertExpectations(t)
}

func TestAddWorkspaceAdminServiceKeycloakFailureIsServerError(t *testing.T) {
	t.Parallel()

	adminClaims := authn.Claims{Username: "admin-user"}
	adminClaims.Subject = "admin-subject"
	workspaceID := "ws-1"
	targetUsername := "target-user"

	workspace := &ws_manager.WorkspaceSettings{Name: workspaceID}
	group := &ws_services.Group{ID: "group-1", Name: workspaceID}
	targetUser := &ws_services.User{ID: "target-id", Username: targetUsername}

	mockKC := new(MockKeycloakClient)
	mockKC.On("GetUserGroups", "admin-subject").Return([]string{workspaceID}, nil)
	mockKC.On("GetGroup", workspaceID).Return(group, nil)
	mockKC.On("GetUser", targetUsername).Return(targetUser, nil)
	mockKC.On("GetGroupMembers", "group-1").Return([]ws_services.User{}, errors.New("keycloak unreachable"))

	mockDB := new(MockWorkspaceDB)
	mockDB.On("IsUserAccountOwner", "admin-user", workspaceID).Return(true, nil)
	mockDB.On("GetWorkspace", workspaceID).Return(workspace, nil)

	svc := &WorkspaceService{DB: mockDB, KC: mockKC}

	rec := httptest.NewRecorder()
	svc.AddWorkspaceAdminService(rec, newWorkspaceAdminRequest(http.MethodPut, workspaceID, targetUsername, adminClaims))

	require.Equal(t, http.StatusInternalServerError, rec.Code)
	mockDB.AssertExpectations(t)
	mockDB.AssertNotCalled(t, "AddWorkspaceAdmin")
	mockKC.AssertExpectations(t)
}

func TestRemoveWorkspaceAdminServiceSuccess(t *testing.T) {
	t.Parallel()

	ownerClaims := authn.Claims{Username: "owner-user"}
	ownerClaims.Subject = "owner-subject"
	workspaceID := "ws-1"
	targetUsername := "target-user"

	mockKC := new(MockKeycloakClient)
	mockKC.On("GetUserGroups", "owner-subject").Return([]string{workspaceID}, nil)

	mockDB := new(MockWorkspaceDB)
	mockDB.On("IsUserAccountOwner", "owner-user", workspaceID).Return(true, nil)
	mockDB.On("RemoveWorkspaceAdmin", targetUsername, workspaceID).Return(nil)

	svc := &WorkspaceService{DB: mockDB, KC: mockKC}

	rec := httptest.NewRecorder()
	svc.RemoveWorkspaceAdminService(rec, newWorkspaceAdminRequest(http.MethodDelete, workspaceID, targetUsername, ownerClaims))

	require.Equal(t, http.StatusNoContent, rec.Code)
	mockDB.AssertExpectations(t)
	mockKC.AssertExpectations(t)
}

func TestRemoveWorkspaceAdminServiceForbiddenForNonAdmin(t *testing.T) {
	t.Parallel()

	memberClaims := authn.Claims{Username: "member-user"}
	memberClaims.Subject = "member-subject"
	workspaceID := "ws-1"
	targetUsername := "target-user"

	mockKC := new(MockKeycloakClient)
	mockKC.On("GetUserGroups", "member-subject").Return([]string{workspaceID}, nil)

	mockDB := new(MockWorkspaceDB)
	mockDB.On("IsUserAccountOwner", "member-user", workspaceID).Return(false, nil)
	mockDB.On("IsUserWorkspaceAdmin", "member-user", workspaceID).Return(false, nil)

	svc := &WorkspaceService{DB: mockDB, KC: mockKC}

	rec := httptest.NewRecorder()
	svc.RemoveWorkspaceAdminService(rec, newWorkspaceAdminRequest(http.MethodDelete, workspaceID, targetUsername, memberClaims))

	require.Equal(t, http.StatusForbidden, rec.Code)
	mockDB.AssertExpectations(t)
	mockKC.AssertExpectations(t)
}
