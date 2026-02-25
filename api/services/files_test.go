package services

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime/multipart"
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

func TestListFilesServiceBlockStoreDownstreamErrorReturnsInternalServerError(t *testing.T) {
	mockDB := new(MockWorkspaceDB)
	mockKC := new(MockKeycloakClient)

	claims := hubAdminClaims()
	workspaceID := "ws-1"
	blockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/ws-1/", r.URL.Path)
		w.WriteHeader(http.StatusInternalServerError)
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

	require.Equal(t, http.StatusInternalServerError, w.Result().StatusCode)
	mockDB.AssertExpectations(t)
}

func TestUploadFilesServiceNoFilesReturnsBadRequest(t *testing.T) {
	mockDB := new(MockWorkspaceDB)
	claims := hubAdminClaims()
	workspaceID := "ws-1"
	workspace := workspaceWithBlockStore(workspaceID)
	mockDB.On("GetWorkspace", workspaceID).Return(workspace, nil).Once()

	svc := FileService{
		DB: mockDB,
	}
	req := newMultipartWorkspaceRequest(t, http.MethodPost, workspaceID, "", []byte(""), &claims)
	w := httptest.NewRecorder()

	svc.UploadFilesService(w, req, storeTypeBlock)

	require.Equal(t, http.StatusBadRequest, w.Result().StatusCode)
	mockDB.AssertExpectations(t)
}

func TestUploadFilesServiceInvalidStoreReturnsBadRequest(t *testing.T) {
	mockDB := new(MockWorkspaceDB)
	claims := hubAdminClaims()
	workspaceID := "ws-1"
	workspace := workspaceWithBlockStore(workspaceID)
	mockDB.On("GetWorkspace", workspaceID).Return(workspace, nil).Once()

	svc := FileService{
		DB: mockDB,
	}
	req := newMultipartWorkspaceRequest(t, http.MethodPost, workspaceID, "upload.tif", []byte("abc"), &claims)
	w := httptest.NewRecorder()

	svc.UploadFilesService(w, req, "invalid")

	require.Equal(t, http.StatusBadRequest, w.Result().StatusCode)
	mockDB.AssertExpectations(t)
}

func TestUploadFilesServiceBlockWithoutStoreReturnsBadRequest(t *testing.T) {
	mockDB := new(MockWorkspaceDB)
	claims := hubAdminClaims()
	workspaceID := "ws-1"
	workspace := &ws_manager.WorkspaceSettings{Name: workspaceID}
	mockDB.On("GetWorkspace", workspaceID).Return(workspace, nil).Once()

	svc := FileService{
		DB: mockDB,
	}
	req := newMultipartWorkspaceRequest(t, http.MethodPost, workspaceID, "upload.tif", []byte("abc"), &claims)
	w := httptest.NewRecorder()

	svc.UploadFilesService(w, req, storeTypeBlock)

	require.Equal(t, http.StatusBadRequest, w.Result().StatusCode)
	mockDB.AssertExpectations(t)
}

func TestUploadFilesServiceBlockSuccess(t *testing.T) {
	mockDB := new(MockWorkspaceDB)
	claims := hubAdminClaims()
	workspaceID := "ws-1"
	workspace := workspaceWithBlockStore(workspaceID)
	mockDB.On("GetWorkspace", workspaceID).Return(workspace, nil).Once()

	blockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPut, r.Method)
		require.Equal(t, "/ws-1/upload.tif", r.URL.Path)
		w.WriteHeader(http.StatusCreated)
	}))
	defer blockServer.Close()

	svc := FileService{
		DB: mockDB,
		Config: &appconfig.Config{
			Files: appconfig.FilesConfig{
				BlockBaseURL: blockServer.URL,
			},
		},
	}
	req := newMultipartWorkspaceRequest(t, http.MethodPost, workspaceID, "upload.tif", []byte("abc"), &claims)
	w := httptest.NewRecorder()

	svc.UploadFilesService(w, req, storeTypeBlock)

	require.Equal(t, http.StatusCreated, w.Result().StatusCode)
	var resp FileUploadResponse
	require.NoError(t, json.NewDecoder(w.Result().Body).Decode(&resp))
	require.Equal(t, workspaceID, resp.Workspace)
	require.Len(t, resp.Items, 1)
	require.Equal(t, "upload.tif", resp.Items[0].FileName)
	mockDB.AssertExpectations(t)
}

func TestDeleteFilesServiceValidatesFileParam(t *testing.T) {
	mockDB := new(MockWorkspaceDB)
	claims := hubAdminClaims()
	workspaceID := "ws-1"
	workspace := workspaceWithBlockStore(workspaceID)
	mockDB.On("GetWorkspace", workspaceID).Return(workspace, nil).Twice()

	svc := FileService{DB: mockDB}

	reqMissing := newWorkspaceRequest(http.MethodDelete, workspaceID, "", nil, &claims)
	wMissing := httptest.NewRecorder()
	svc.DeleteFilesService(wMissing, reqMissing, storeTypeBlock)
	require.Equal(t, http.StatusBadRequest, wMissing.Result().StatusCode)

	reqInvalid := newWorkspaceRequest(http.MethodDelete, workspaceID, "file=bad/name.tif", nil, &claims)
	wInvalid := httptest.NewRecorder()
	svc.DeleteFilesService(wInvalid, reqInvalid, storeTypeBlock)
	require.Equal(t, http.StatusBadRequest, wInvalid.Result().StatusCode)

	mockDB.AssertExpectations(t)
}

func TestDeleteFilesServiceBlockConflictAndSuccess(t *testing.T) {
	mockDB := new(MockWorkspaceDB)
	claims := hubAdminClaims()
	workspaceID := "ws-1"
	workspace := workspaceWithBlockStore(workspaceID)
	mockDB.On("GetWorkspace", workspaceID).Return(workspace, nil).Twice()

	blockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodDelete, r.Method)
		switch r.URL.Path {
		case "/ws-1/missing.tif":
			w.WriteHeader(http.StatusNotFound)
		case "/ws-1/good.tif":
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer blockServer.Close()

	svc := FileService{
		DB: mockDB,
		Config: &appconfig.Config{
			Files: appconfig.FilesConfig{
				BlockBaseURL: blockServer.URL,
			},
		},
	}

	reqConflict := newWorkspaceRequest(http.MethodDelete, workspaceID, "file=missing.tif", nil, &claims)
	wConflict := httptest.NewRecorder()
	svc.DeleteFilesService(wConflict, reqConflict, storeTypeBlock)
	require.Equal(t, http.StatusConflict, wConflict.Result().StatusCode)
	var conflictResp FileDeleteResponse
	require.NoError(t, json.NewDecoder(wConflict.Result().Body).Decode(&conflictResp))
	require.Len(t, conflictResp.Failed, 1)

	reqOK := newWorkspaceRequest(http.MethodDelete, workspaceID, "file=good.tif", nil, &claims)
	wOK := httptest.NewRecorder()
	svc.DeleteFilesService(wOK, reqOK, storeTypeBlock)
	require.Equal(t, http.StatusOK, wOK.Result().StatusCode)
	var okResp FileDeleteResponse
	require.NoError(t, json.NewDecoder(wOK.Result().Body).Decode(&okResp))
	require.Equal(t, []string{"good.tif"}, okResp.Deleted)

	mockDB.AssertExpectations(t)
}

func TestGetFileMetadataServiceValidatesFileParam(t *testing.T) {
	mockDB := new(MockWorkspaceDB)
	claims := hubAdminClaims()
	workspaceID := "ws-1"
	workspace := workspaceWithBlockStore(workspaceID)
	mockDB.On("GetWorkspace", workspaceID).Return(workspace, nil).Twice()

	svc := FileService{DB: mockDB}

	reqMissing := newWorkspaceRequest(http.MethodGet, workspaceID, "", nil, &claims)
	wMissing := httptest.NewRecorder()
	svc.GetFileMetadataService(wMissing, reqMissing, storeTypeBlock)
	require.Equal(t, http.StatusBadRequest, wMissing.Result().StatusCode)

	reqInvalid := newWorkspaceRequest(http.MethodGet, workspaceID, "file=bad/name.tif", nil, &claims)
	wInvalid := httptest.NewRecorder()
	svc.GetFileMetadataService(wInvalid, reqInvalid, storeTypeBlock)
	require.Equal(t, http.StatusBadRequest, wInvalid.Result().StatusCode)

	mockDB.AssertExpectations(t)
}

func TestGetFileMetadataServiceBlockNotFoundAndSuccess(t *testing.T) {
	mockDB := new(MockWorkspaceDB)
	claims := hubAdminClaims()
	workspaceID := "ws-1"
	workspace := workspaceWithBlockStore(workspaceID)
	mockDB.On("GetWorkspace", workspaceID).Return(workspace, nil).Twice()

	blockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodHead, r.Method)
		switch r.URL.Path {
		case "/ws-1/missing.tif":
			w.WriteHeader(http.StatusNotFound)
		case "/ws-1/good.tif":
			w.Header().Set("Content-Length", "12")
			w.Header().Set("Last-Modified", "Wed, 11 Feb 2026 12:53:04 GMT")
			w.Header().Set("ETag", `"etag-1"`)
			w.WriteHeader(http.StatusOK)
		default:
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer blockServer.Close()

	svc := FileService{
		DB: mockDB,
		Config: &appconfig.Config{
			Files: appconfig.FilesConfig{
				BlockBaseURL: blockServer.URL,
			},
		},
	}

	reqNotFound := newWorkspaceRequest(http.MethodGet, workspaceID, "file=missing.tif", nil, &claims)
	wNotFound := httptest.NewRecorder()
	svc.GetFileMetadataService(wNotFound, reqNotFound, storeTypeBlock)
	require.Equal(t, http.StatusNotFound, wNotFound.Result().StatusCode)

	reqOK := newWorkspaceRequest(http.MethodGet, workspaceID, "file=good.tif", nil, &claims)
	wOK := httptest.NewRecorder()
	svc.GetFileMetadataService(wOK, reqOK, storeTypeBlock)
	require.Equal(t, http.StatusOK, wOK.Result().StatusCode)
	var resp FileMetadataResponse
	require.NoError(t, json.NewDecoder(wOK.Result().Body).Decode(&resp))
	require.Equal(t, workspaceID, resp.Workspace)
	require.Equal(t, "good.tif", resp.Item.FileName)
	require.Equal(t, int64(12), resp.Item.Size)

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

func newWorkspaceRequest(method, workspaceID, query string, body io.Reader, claims *authn.Claims) *http.Request {
	url := "/api/workspaces/" + workspaceID + "/files"
	if query != "" {
		url = url + "?" + query
	}

	req := httptest.NewRequest(method, url, body)
	req = mux.SetURLVars(req, map[string]string{"workspace-id": workspaceID})
	if claims == nil {
		return req
	}

	ctx := context.WithValue(req.Context(), middleware.ClaimsKey, *claims)
	return req.WithContext(ctx)
}

func newMultipartWorkspaceRequest(
	t *testing.T,
	method string,
	workspaceID string,
	fileName string,
	data []byte,
	claims *authn.Claims,
) *http.Request {
	t.Helper()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if fileName != "" {
		part, err := writer.CreateFormFile("files", fileName)
		require.NoError(t, err)
		_, err = part.Write(data)
		require.NoError(t, err)
	}
	require.NoError(t, writer.Close())

	req := newWorkspaceRequest(method, workspaceID, "", &body, claims)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req
}

func workspaceWithBlockStore(workspaceID string) *ws_manager.WorkspaceSettings {
	stores := []ws_manager.Stores{
		{
			Block: []ws_manager.BlockStore{
				{MountPoint: "/" + workspaceID},
			},
		},
	}
	return &ws_manager.WorkspaceSettings{
		Name:   workspaceID,
		Stores: &stores,
	}
}

// hubAdminClaims bypasses workspace membership checks so tests can focus on
// specific handler/service behavior (validation and not-found paths).
func hubAdminClaims() authn.Claims {
	claims := authn.Claims{Username: "admin-user"}
	claims.Subject = "admin-subject"
	claims.RealmAccess.Roles = []string{"hub_admin"}
	return claims
}
