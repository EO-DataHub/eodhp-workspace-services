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

func newUserRequest(method, workspaceID, username string, claims authn.Claims) *http.Request {
	url := "/workspaces/" + workspaceID + "/users"
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

func TestAddUserServiceSuccess(t *testing.T) {
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
	mockKC.On("AddMemberToGroup", "target-id", "group-1").Return(nil)

	mockDB := new(MockWorkspaceDB)
	mockDB.On("IsUserAccountOwner", "admin-user", workspaceID).Return(true, nil)
	mockDB.On("GetWorkspace", workspaceID).Return(workspace, nil)
	mockDB.On("RemoveWorkspaceAdmin", targetUsername, workspaceID).Return(nil)

	svc := &WorkspaceService{DB: mockDB, KC: mockKC}

	rec := httptest.NewRecorder()
	svc.AddUserService(rec, newUserRequest(http.MethodPut, workspaceID, targetUsername, adminClaims))

	require.Equal(t, http.StatusNoContent, rec.Code)
	mockDB.AssertExpectations(t)
	mockKC.AssertExpectations(t)
}

func TestAddUserServiceForbiddenForNonAdmin(t *testing.T) {
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
	svc.AddUserService(rec, newUserRequest(http.MethodPut, workspaceID, targetUsername, memberClaims))

	require.Equal(t, http.StatusForbidden, rec.Code)
	mockDB.AssertExpectations(t)
	mockDB.AssertNotCalled(t, "RemoveWorkspaceAdmin")
	mockKC.AssertExpectations(t)
}

func TestAddUserServiceUserNotFound(t *testing.T) {
	t.Parallel()

	adminClaims := authn.Claims{Username: "admin-user"}
	adminClaims.Subject = "admin-subject"
	workspaceID := "ws-1"
	targetUsername := "missing-user"

	workspace := &ws_manager.WorkspaceSettings{Name: workspaceID}
	group := &ws_services.Group{ID: "group-1", Name: workspaceID}

	mockKC := new(MockKeycloakClient)
	mockKC.On("GetUserGroups", "admin-subject").Return([]string{workspaceID}, nil)
	mockKC.On("GetGroup", workspaceID).Return(group, nil)
	mockKC.On("GetUser", targetUsername).Return(&ws_services.User{}, errors.New("user not found"))

	mockDB := new(MockWorkspaceDB)
	mockDB.On("IsUserAccountOwner", "admin-user", workspaceID).Return(true, nil)
	mockDB.On("GetWorkspace", workspaceID).Return(workspace, nil)

	svc := &WorkspaceService{DB: mockDB, KC: mockKC}

	rec := httptest.NewRecorder()
	svc.AddUserService(rec, newUserRequest(http.MethodPut, workspaceID, targetUsername, adminClaims))

	require.Equal(t, http.StatusNotFound, rec.Code)
	mockDB.AssertExpectations(t)
	mockDB.AssertNotCalled(t, "RemoveWorkspaceAdmin")
	mockKC.AssertExpectations(t)
	mockKC.AssertNotCalled(t, "AddMemberToGroup")
}

func TestAddUserServiceKeycloakAddFailureIsServerError(t *testing.T) {
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
	mockKC.On("AddMemberToGroup", "target-id", "group-1").Return(errors.New("keycloak unreachable"))

	mockDB := new(MockWorkspaceDB)
	mockDB.On("IsUserAccountOwner", "admin-user", workspaceID).Return(true, nil)
	mockDB.On("GetWorkspace", workspaceID).Return(workspace, nil)

	svc := &WorkspaceService{DB: mockDB, KC: mockKC}

	rec := httptest.NewRecorder()
	svc.AddUserService(rec, newUserRequest(http.MethodPut, workspaceID, targetUsername, adminClaims))

	require.Equal(t, http.StatusInternalServerError, rec.Code)
	mockDB.AssertExpectations(t)
	mockDB.AssertNotCalled(t, "RemoveWorkspaceAdmin")
	mockKC.AssertExpectations(t)
}

func TestAddUserServiceClearStaleAdminFailureIsServerError(t *testing.T) {
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
	mockKC.On("AddMemberToGroup", "target-id", "group-1").Return(nil)

	mockDB := new(MockWorkspaceDB)
	mockDB.On("IsUserAccountOwner", "admin-user", workspaceID).Return(true, nil)
	mockDB.On("GetWorkspace", workspaceID).Return(workspace, nil)
	mockDB.On("RemoveWorkspaceAdmin", targetUsername, workspaceID).Return(errors.New("db unreachable"))

	svc := &WorkspaceService{DB: mockDB, KC: mockKC}

	rec := httptest.NewRecorder()
	svc.AddUserService(rec, newUserRequest(http.MethodPut, workspaceID, targetUsername, adminClaims))

	require.Equal(t, http.StatusInternalServerError, rec.Code)
	mockDB.AssertExpectations(t)
	mockKC.AssertExpectations(t)
}

func TestRemoveUserServiceSuccess(t *testing.T) {
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
	mockKC.On("RemoveMemberFromGroup", "target-id", "group-1").Return(nil, nil)

	mockDB := new(MockWorkspaceDB)
	mockDB.On("IsUserAccountOwner", "admin-user", workspaceID).Return(true, nil)
	mockDB.On("IsUserAccountOwner", targetUsername, workspaceID).Return(false, nil)
	mockDB.On("GetWorkspace", workspaceID).Return(workspace, nil)
	mockDB.On("RemoveWorkspaceAdmin", targetUsername, workspaceID).Return(nil)

	svc := &WorkspaceService{DB: mockDB, KC: mockKC}

	rec := httptest.NewRecorder()
	svc.RemoveUserService(rec, newUserRequest(http.MethodDelete, workspaceID, targetUsername, adminClaims))

	require.Equal(t, http.StatusNoContent, rec.Code)
	mockDB.AssertExpectations(t)
	mockKC.AssertExpectations(t)
}

func TestRemoveUserServiceForbiddenForNonAdmin(t *testing.T) {
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
	svc.RemoveUserService(rec, newUserRequest(http.MethodDelete, workspaceID, targetUsername, memberClaims))

	require.Equal(t, http.StatusForbidden, rec.Code)
	mockDB.AssertExpectations(t)
	mockDB.AssertNotCalled(t, "RemoveWorkspaceAdmin")
	mockKC.AssertExpectations(t)
}

func TestRemoveUserServiceForbiddenForAccountOwnerSelfRemoval(t *testing.T) {
	t.Parallel()

	ownerClaims := authn.Claims{Username: "owner-user"}
	ownerClaims.Subject = "owner-subject"
	workspaceID := "ws-1"

	mockKC := new(MockKeycloakClient)
	mockKC.On("GetUserGroups", "owner-subject").Return([]string{workspaceID}, nil)

	mockDB := new(MockWorkspaceDB)
	mockDB.On("IsUserAccountOwner", "owner-user", workspaceID).Return(true, nil)

	svc := &WorkspaceService{DB: mockDB, KC: mockKC}

	rec := httptest.NewRecorder()
	svc.RemoveUserService(rec, newUserRequest(http.MethodDelete, workspaceID, "owner-user", ownerClaims))

	require.Equal(t, http.StatusForbidden, rec.Code)
	mockDB.AssertExpectations(t)
	mockDB.AssertNotCalled(t, "RemoveWorkspaceAdmin")
	mockKC.AssertExpectations(t)
	mockKC.AssertNotCalled(t, "RemoveMemberFromGroup")
}

func TestRemoveUserServiceKeycloakRemoveFailureIsServerError(t *testing.T) {
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
	mockKC.On("RemoveMemberFromGroup", "target-id", "group-1").Return(nil, errors.New("keycloak unreachable"))

	mockDB := new(MockWorkspaceDB)
	mockDB.On("IsUserAccountOwner", "admin-user", workspaceID).Return(true, nil)
	mockDB.On("IsUserAccountOwner", targetUsername, workspaceID).Return(false, nil)
	mockDB.On("GetWorkspace", workspaceID).Return(workspace, nil)

	svc := &WorkspaceService{DB: mockDB, KC: mockKC}

	rec := httptest.NewRecorder()
	svc.RemoveUserService(rec, newUserRequest(http.MethodDelete, workspaceID, targetUsername, adminClaims))

	require.Equal(t, http.StatusInternalServerError, rec.Code)
	mockDB.AssertExpectations(t)
	mockDB.AssertNotCalled(t, "RemoveWorkspaceAdmin")
	mockKC.AssertExpectations(t)
}

func TestRemoveUserServiceDBRemoveFailureIsServerError(t *testing.T) {
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
	mockKC.On("RemoveMemberFromGroup", "target-id", "group-1").Return(nil, nil)

	mockDB := new(MockWorkspaceDB)
	mockDB.On("IsUserAccountOwner", "admin-user", workspaceID).Return(true, nil)
	mockDB.On("IsUserAccountOwner", targetUsername, workspaceID).Return(false, nil)
	mockDB.On("GetWorkspace", workspaceID).Return(workspace, nil)
	mockDB.On("RemoveWorkspaceAdmin", targetUsername, workspaceID).Return(errors.New("db unreachable"))

	svc := &WorkspaceService{DB: mockDB, KC: mockKC}

	rec := httptest.NewRecorder()
	svc.RemoveUserService(rec, newUserRequest(http.MethodDelete, workspaceID, targetUsername, adminClaims))

	require.Equal(t, http.StatusInternalServerError, rec.Code)
	mockDB.AssertExpectations(t)
	mockKC.AssertExpectations(t)
}
