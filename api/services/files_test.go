package services

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	ws_manager "github.com/EO-DataHub/eodhp-workspace-manager/models"
	"github.com/EO-DataHub/eodhp-workspace-services/api/middleware"
	"github.com/EO-DataHub/eodhp-workspace-services/internal/appconfig"
	"github.com/EO-DataHub/eodhp-workspace-services/internal/authn"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/require"
)

func TestValidateFileName(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{name: "valid", input: "file.tif", wantErr: false},
		{name: "valid with dash", input: "my-file_01.txt", wantErr: false},
		{name: "empty", input: "", wantErr: true},
		{name: "dot", input: ".", wantErr: true},
		{name: "slash", input: "dir/file.tif", wantErr: true},
		{name: "backslash", input: "dir\\file.tif", wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateFileName(tc.input)
			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestListFilesServiceMissingClaimsReturnsUnauthorized(t *testing.T) {
	svc := FileService{}
	req := newListFilesRequest("ws-1", "", nil)
	w := httptest.NewRecorder()

	svc.ListFilesService(w, req)

	require.Equal(t, http.StatusUnauthorized, w.Result().StatusCode)
}

func TestListFilesServiceNonMemberReturnsForbidden(t *testing.T) {
	mockDB := new(MockWorkspaceDB)
	mockKC := new(MockKeycloakClient)

	claims := authn.Claims{Username: "dev-user"}
	claims.Subject = "user-123"

	mockKC.On("GetUserGroups", "user-123").Return([]string{"another-workspace"}, nil).Once()

	svc := FileService{
		DB: mockDB,
		KC: mockKC,
	}
	req := newListFilesRequest("ws-1", "", &claims)
	w := httptest.NewRecorder()

	svc.ListFilesService(w, req)

	require.Equal(t, http.StatusForbidden, w.Result().StatusCode)
	mockKC.AssertExpectations(t)
}

func TestListFilesServiceWorkspaceNotFoundReturnsNotFound(t *testing.T) {
	mockDB := new(MockWorkspaceDB)
	mockKC := new(MockKeycloakClient)

	claims := hubAdminClaims()
	workspaceID := "missing-ws"

	mockDB.On("GetWorkspace", workspaceID).
		Return(&ws_manager.WorkspaceSettings{}, errors.New("not found")).Once()

	svc := FileService{
		DB: mockDB,
		KC: mockKC,
	}
	req := newListFilesRequest(workspaceID, "", &claims)
	w := httptest.NewRecorder()

	svc.ListFilesService(w, req)

	require.Equal(t, http.StatusNotFound, w.Result().StatusCode)
	mockDB.AssertExpectations(t)
}

func TestListFilesServiceInvalidStoreReturnsBadRequest(t *testing.T) {
	mockDB := new(MockWorkspaceDB)
	mockKC := new(MockKeycloakClient)

	claims := hubAdminClaims()
	workspaceID := "ws-1"
	workspace := &ws_manager.WorkspaceSettings{Name: workspaceID}

	mockDB.On("GetWorkspace", workspaceID).Return(workspace, nil).Once()

	svc := FileService{
		DB: mockDB,
		KC: mockKC,
	}
	req := newListFilesRequest(workspaceID, "store=invalid", &claims)
	w := httptest.NewRecorder()

	svc.ListFilesService(w, req)

	require.Equal(t, http.StatusBadRequest, w.Result().StatusCode)
	mockDB.AssertExpectations(t)
}

func TestListFilesServiceBlockStoreMissingDirectoryReturnsEmptyItems(t *testing.T) {
	mockDB := new(MockWorkspaceDB)
	mockKC := new(MockKeycloakClient)

	claims := hubAdminClaims()
	workspaceID := "ws-1"
	blockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/ws-1/", r.URL.Path)
		w.WriteHeader(http.StatusNotFound)
	}))
	defer blockServer.Close()

	stores := []ws_manager.Stores{
		{
			Block: []ws_manager.BlockStore{
				{Name: "block-store", MountPoint: "/ws-1"},
			},
		},
	}
	workspace := &ws_manager.WorkspaceSettings{
		Name:   workspaceID,
		Stores: &stores,
	}

	mockDB.On("GetWorkspace", workspaceID).Return(workspace, nil).Once()

	svc := FileService{
		DB: mockDB,
		KC: mockKC,
		Config: &appconfig.Config{
			Files: appconfig.FilesConfig{
				BlockBaseURL: blockServer.URL,
			},
		},
	}
	req := newListFilesRequest(workspaceID, "store=block", &claims)
	w := httptest.NewRecorder()

	svc.ListFilesService(w, req)

	require.Equal(t, http.StatusOK, w.Result().StatusCode)

	var resp FileListResponse
	require.NoError(t, json.NewDecoder(w.Result().Body).Decode(&resp))
	require.Equal(t, workspaceID, resp.Workspace)
	require.Len(t, resp.Items, 0)
	mockDB.AssertExpectations(t)
}

func TestResponseTimeFormatDefaultsWhenConfigIsMissing(t *testing.T) {
	svc := FileService{}
	require.Equal(t, defaultTimeFormat, svc.responseTimeFormat())
}

func TestResponseTimeFormatUsesConfiguredValue(t *testing.T) {
	svc := FileService{
		Config: &appconfig.Config{
			Files: appconfig.FilesConfig{
				ResponseTimeFormat: "2006-01-02",
			},
		},
	}
	require.Equal(t, "2006-01-02", svc.responseTimeFormat())
}

func TestMaxUploadFormMemoryBytesDefaultsWhenConfigIsMissing(t *testing.T) {
	svc := FileService{}
	require.Equal(t, defaultFormMemory, svc.maxUploadFormMemoryBytes())
}

func TestMaxUploadFormMemoryBytesUsesConfiguredValue(t *testing.T) {
	svc := FileService{
		Config: &appconfig.Config{
			Files: appconfig.FilesConfig{
				MaxUploadFormMemory: 64,
			},
		},
	}
	require.Equal(t, int64(64<<20), svc.maxUploadFormMemoryBytes())
}

func newListFilesRequest(workspaceID, query string, claims *authn.Claims) *http.Request {
	url := "/api/workspaces/" + workspaceID + "/files"
	if query != "" {
		url = url + "?" + query
	}

	req := httptest.NewRequest(http.MethodGet, url, nil)
	req = mux.SetURLVars(req, map[string]string{"workspace-id": workspaceID})
	if claims == nil {
		return req
	}

	ctx := context.WithValue(req.Context(), middleware.ClaimsKey, *claims)
	return req.WithContext(ctx)
}

// hubAdminClaims bypasses workspace membership checks so tests can focus on
// specific handler/service behavior (validation and not-found paths).
func hubAdminClaims() authn.Claims {
	claims := authn.Claims{Username: "admin-user"}
	claims.Subject = "admin-subject"
	claims.RealmAccess.Roles = []string{"hub_admin"}
	return claims
}
