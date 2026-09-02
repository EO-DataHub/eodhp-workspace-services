package services

import (
	"testing"

	"github.com/EO-DataHub/eodhp-workspace-services/internal/authn"
	"github.com/stretchr/testify/require"
)

func TestIsUserWorkspaceAuthorizedHubAdmin(t *testing.T) {
	t.Parallel()

	claims := authn.Claims{Username: "any-user"}
	claims.RealmAccess.Roles = []string{"hub_admin"}

	mockDB := new(MockWorkspaceDB)
	mockKC := new(MockKeycloakClient)

	authorized, err := isUserWorkspaceAuthorized(mockDB, mockKC, claims, "ws-1", true)
	require.NoError(t, err)
	require.True(t, authorized)

	mockDB.AssertExpectations(t)
	mockKC.AssertExpectations(t)
}

func TestIsUserWorkspaceAuthorizedServiceAccount(t *testing.T) {
	t.Parallel()

	claims := authn.Claims{Username: "service-account-eodh-workspaces"}

	mockDB := new(MockWorkspaceDB)
	mockKC := new(MockKeycloakClient)

	authorized, err := isUserWorkspaceAuthorized(mockDB, mockKC, claims, "ws-1", true)
	require.NoError(t, err)
	require.True(t, authorized)

	mockDB.AssertExpectations(t)
	mockKC.AssertExpectations(t)
}

func TestIsUserWorkspaceAuthorizedMemberOnly(t *testing.T) {
	t.Parallel()

	claims := authn.Claims{Username: "member-user"}
	claims.Subject = "member-subject"

	mockDB := new(MockWorkspaceDB)
	mockKC := new(MockKeycloakClient)
	mockKC.On("GetUserGroups", "member-subject").Return([]string{"ws-1"}, nil)

	authorized, err := isUserWorkspaceAuthorized(mockDB, mockKC, claims, "ws-1", false)
	require.NoError(t, err)
	require.True(t, authorized)

	mockDB.AssertExpectations(t)
	mockKC.AssertExpectations(t)
}

func TestIsUserWorkspaceAuthorizedNonMember(t *testing.T) {
	t.Parallel()

	claims := authn.Claims{Username: "outsider"}
	claims.Subject = "outsider-subject"

	mockDB := new(MockWorkspaceDB)
	mockKC := new(MockKeycloakClient)
	mockKC.On("GetUserGroups", "outsider-subject").Return([]string{"ws-other"}, nil)

	authorized, err := isUserWorkspaceAuthorized(mockDB, mockKC, claims, "ws-1", false)
	require.NoError(t, err)
	require.False(t, authorized)

	mockDB.AssertExpectations(t)
	mockKC.AssertExpectations(t)
}

func TestIsUserWorkspaceAuthorizedAccountOwnerIsImplicitAdmin(t *testing.T) {
	t.Parallel()

	claims := authn.Claims{Username: "owner-user"}
	claims.Subject = "owner-subject"

	mockDB := new(MockWorkspaceDB)
	mockDB.On("IsUserAccountOwner", "owner-user", "ws-1").Return(true, nil)

	mockKC := new(MockKeycloakClient)
	mockKC.On("GetUserGroups", "owner-subject").Return([]string{"ws-1"}, nil)

	authorized, err := isUserWorkspaceAuthorized(mockDB, mockKC, claims, "ws-1", true)
	require.NoError(t, err)
	require.True(t, authorized)

	// The account owner is authorized without needing an explicit workspace_admins row.
	mockDB.AssertNotCalled(t, "IsUserWorkspaceAdmin", "owner-user", "ws-1")
	mockDB.AssertExpectations(t)
	mockKC.AssertExpectations(t)
}

func TestIsUserWorkspaceAuthorizedWorkspaceAdmin(t *testing.T) {
	t.Parallel()

	claims := authn.Claims{Username: "admin-user"}
	claims.Subject = "admin-subject"

	mockDB := new(MockWorkspaceDB)
	mockDB.On("IsUserAccountOwner", "admin-user", "ws-1").Return(false, nil)
	mockDB.On("IsUserWorkspaceAdmin", "admin-user", "ws-1").Return(true, nil)

	mockKC := new(MockKeycloakClient)
	mockKC.On("GetUserGroups", "admin-subject").Return([]string{"ws-1"}, nil)

	authorized, err := isUserWorkspaceAuthorized(mockDB, mockKC, claims, "ws-1", true)
	require.NoError(t, err)
	require.True(t, authorized)

	mockDB.AssertExpectations(t)
	mockKC.AssertExpectations(t)
}

func TestIsUserWorkspaceAuthorizedNeitherOwnerNorAdmin(t *testing.T) {
	t.Parallel()

	claims := authn.Claims{Username: "member-user"}
	claims.Subject = "member-subject"

	mockDB := new(MockWorkspaceDB)
	mockDB.On("IsUserAccountOwner", "member-user", "ws-1").Return(false, nil)
	mockDB.On("IsUserWorkspaceAdmin", "member-user", "ws-1").Return(false, nil)

	mockKC := new(MockKeycloakClient)
	mockKC.On("GetUserGroups", "member-subject").Return([]string{"ws-1"}, nil)

	authorized, err := isUserWorkspaceAuthorized(mockDB, mockKC, claims, "ws-1", true)
	require.NoError(t, err)
	require.False(t, authorized)

	mockDB.AssertExpectations(t)
	mockKC.AssertExpectations(t)
}

func TestIsUserWorkspaceAuthorizedNonMemberCannotBeAdmin(t *testing.T) {
	t.Parallel()

	claims := authn.Claims{Username: "outsider"}
	claims.Subject = "outsider-subject"

	mockDB := new(MockWorkspaceDB)
	mockKC := new(MockKeycloakClient)
	mockKC.On("GetUserGroups", "outsider-subject").Return([]string{"ws-other"}, nil)

	authorized, err := isUserWorkspaceAuthorized(mockDB, mockKC, claims, "ws-1", true)
	require.NoError(t, err)
	require.False(t, authorized)

	// A non-member is rejected without any DB lookups.
	mockDB.AssertExpectations(t)
	mockKC.AssertExpectations(t)
}
