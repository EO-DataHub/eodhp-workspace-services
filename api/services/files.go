package services

import (
	"context"
	"errors"
	"net/http"
	"strings"

	ws_manager "github.com/EO-DataHub/eodhp-workspace-manager/models"
	"github.com/EO-DataHub/eodhp-workspace-services/api/middleware"
	"github.com/EO-DataHub/eodhp-workspace-services/db"
	"github.com/EO-DataHub/eodhp-workspace-services/internal/appconfig"
	"github.com/EO-DataHub/eodhp-workspace-services/internal/authn"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/gorilla/mux"
	"github.com/rs/zerolog"
)

const (
	storeTypeObject   = "object"
	storeTypeBlock    = "block"
	invalidStoreType  = "invalid store type"
	defaultTimeFormat = "2006-01-02T15:04:05Z"
	defaultFormMemory = int64(32 << 20) // 32MB
)

// STSClient defines the minimal interface needed for STS AssumeRoleWithWebIdentity.
type STSClient interface {
	AssumeRoleWithWebIdentity(ctx context.Context,
		params *sts.AssumeRoleWithWebIdentityInput, optFns ...func(*sts.Options)) (
		*sts.AssumeRoleWithWebIdentityOutput, error)
}

type FileService struct {
	Config *appconfig.Config
	DB     db.WorkspaceDBInterface
	KC     KeycloakClientInterface
	STS    STSClient
}

type FileItem struct {
	StoreType    string `json:"storeType"`
	FileName     string `json:"fileName"`
	Size         int64  `json:"size,omitempty"`
	LastModified string `json:"lastModified,omitempty"`
	ETag         string `json:"etag,omitempty"`
}

type FileListResponse struct {
	Workspace string     `json:"workspace"`
	Items     []FileItem `json:"items"`
}

type FileUploadResponse struct {
	Workspace string     `json:"workspace"`
	Items     []FileItem `json:"items"`
}

type FileDeleteResponse struct {
	Workspace string     `json:"workspace"`
	Deleted   []string   `json:"deleted"`
	Failed    []FileFail `json:"failed,omitempty"`
}

type FileFail struct {
	FileName string `json:"fileName"`
	Error    string `json:"error"`
}

type FileMetadataResponse struct {
	Workspace string   `json:"workspace"`
	Item      FileItem `json:"item"`
}

func (svc *FileService) resolveAuthorizedWorkspace(w http.ResponseWriter, r *http.Request) (string, *ws_manager.WorkspaceSettings, bool) {
	logger := zerolog.Ctx(r.Context())

	workspaceID := mux.Vars(r)["workspace-id"]
	claims, ok := r.Context().Value(middleware.ClaimsKey).(authn.Claims)
	if !ok {
		logger.Warn().Msg("Unauthorized request: missing claims")
		WriteResponse(w, http.StatusUnauthorized, "missing claims")
		return "", nil, false
	}

	authorized, err := isUserWorkspaceAuthorized(svc.DB, svc.KC, claims, workspaceID, false)
	if err != nil {
		logger.Error().Err(err).Str("workspace_id", workspaceID).Msg("Failed to authorize workspace")
		WriteResponse(w, http.StatusInternalServerError, "authorization failed")
		return "", nil, false
	}
	if !authorized {
		logger.Warn().Str("workspace_id", workspaceID).Msg("Access denied")
		WriteResponse(w, http.StatusForbidden, "access denied")
		return "", nil, false
	}

	workspace, err := svc.DB.GetWorkspace(workspaceID)
	if err != nil {
		logger.Error().Err(err).Str("workspace_id", workspaceID).Msg("Workspace not found")
		WriteResponse(w, http.StatusNotFound, "workspace not found")
		return "", nil, false
	}

	return workspaceID, workspace, true
}

func resolveStoreSelection(storeType string, allowAll bool) (bool, bool, error) {
	switch strings.ToLower(strings.TrimSpace(storeType)) {
	case "":
		if allowAll {
			return true, true, nil
		}
		return false, false, errors.New(invalidStoreType)
	case storeTypeObject:
		return true, false, nil
	case storeTypeBlock:
		return false, true, nil
	default:
		return false, false, errors.New(invalidStoreType)
	}
}

// ListFilesService lists files from object and/or block stores.
func (svc *FileService) ListFilesService(w http.ResponseWriter, r *http.Request) {
	workspaceID, workspace, ok := svc.resolveAuthorizedWorkspace(w, r)
	if !ok {
		return
	}
	// Propagate the request context so downstream I/O is canceled on client disconnect/timeout.
	ctx := r.Context()

	storeType := r.URL.Query().Get("store")
	var items []FileItem
	objectStores, blockStores := collectStores(workspace)

	wantObject, wantBlock, err := resolveStoreSelection(storeType, true)
	if err != nil {
		WriteResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	if wantObject {
		objItems, err := svc.listObjectStoreItems(r, objectStores)
		if err != nil {
			WriteResponse(w, http.StatusBadRequest, err.Error())
			return
		}
		items = append(items, objItems...)
	}
	if wantBlock {
		blkItems, err := svc.listBlockStoreItems(ctx, blockStores, workspaceID)
		if err != nil {
			WriteResponse(w, http.StatusBadRequest, err.Error())
			return
		}
		items = append(items, blkItems...)
	}

	WriteResponse(w, http.StatusOK, FileListResponse{
		Workspace: workspaceID,
		Items:     items,
	})
}

// UploadFilesService uploads files to a single store.
func (svc *FileService) UploadFilesService(w http.ResponseWriter, r *http.Request, storeType string) {
	workspaceID, workspace, ok := svc.resolveAuthorizedWorkspace(w, r)
	if !ok {
		return
	}
	// Propagate the request context so downstream I/O is canceled on client disconnect/timeout.
	ctx := r.Context()

	if err := r.ParseMultipartForm(svc.maxUploadFormMemoryBytes()); err != nil {
		WriteResponse(w, http.StatusBadRequest, "invalid multipart form data")
		return
	}
	files := collectMultipartFiles(r.MultipartForm)
	if len(files) == 0 {
		WriteResponse(w, http.StatusBadRequest, "no files provided")
		return
	}

	var items []FileItem
	wantObject, _, err := resolveStoreSelection(storeType, false)
	if err != nil {
		WriteResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	if wantObject {
		objectStores, _ := collectStores(workspace)
		objectStore, err := selectObjectStore(objectStores)
		if err != nil {
			WriteResponse(w, http.StatusBadRequest, err.Error())
			return
		}
		uploaded, err := svc.uploadObjectStoreFiles(r, objectStore, files)
		if err != nil {
			WriteResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
		items = uploaded
	} else {
		_, blockStores := collectStores(workspace)
		blockStore, err := selectBlockStore(blockStores)
		if err != nil {
			WriteResponse(w, http.StatusBadRequest, err.Error())
			return
		}
		uploaded, err := svc.uploadBlockStoreFiles(ctx, workspaceID, blockStore, files)
		if err != nil {
			WriteResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
		items = uploaded
	}

	WriteResponse(w, http.StatusCreated, FileUploadResponse{
		Workspace: workspaceID,
		Items:     items,
	})
}

// DeleteFilesService deletes files from a single store.
func (svc *FileService) DeleteFilesService(w http.ResponseWriter, r *http.Request, storeType string) {
	workspaceID, workspace, ok := svc.resolveAuthorizedWorkspace(w, r)
	if !ok {
		return
	}
	// Propagate the request context so downstream I/O is canceled on client disconnect/timeout.
	ctx := r.Context()

	fileName := r.URL.Query().Get("file")
	if fileName == "" {
		WriteResponse(w, http.StatusBadRequest, "file is required")
		return
	}
	if err := validateFileName(fileName); err != nil {
		WriteResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	var deleted []string
	var failed []FileFail

	wantObject, _, err := resolveStoreSelection(storeType, false)
	if err != nil {
		WriteResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	if wantObject {
		objectStores, _ := collectStores(workspace)
		objectStore, err := selectObjectStore(objectStores)
		if err != nil {
			WriteResponse(w, http.StatusBadRequest, err.Error())
			return
		}
		deleted, failed, err = svc.deleteObjectStoreFiles(r, objectStore, []string{fileName})
		if err != nil {
			WriteResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
	} else {
		_, blockStores := collectStores(workspace)
		blockStore, err := selectBlockStore(blockStores)
		if err != nil {
			WriteResponse(w, http.StatusBadRequest, err.Error())
			return
		}
		deleted, failed, err = svc.deleteBlockStoreFiles(ctx, workspaceID, blockStore, []string{fileName})
		if err != nil {
			WriteResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
	}

	status := http.StatusOK
	if len(failed) > 0 {
		status = http.StatusConflict
	}

	WriteResponse(w, status, FileDeleteResponse{
		Workspace: workspaceID,
		Deleted:   deleted,
		Failed:    failed,
	})
}

// GetFileMetadataService gets metadata for a single file.
func (svc *FileService) GetFileMetadataService(w http.ResponseWriter, r *http.Request, storeType string) {
	workspaceID, workspace, ok := svc.resolveAuthorizedWorkspace(w, r)
	if !ok {
		return
	}
	// Propagate the request context so downstream I/O is canceled on client disconnect/timeout.
	ctx := r.Context()

	fileName := r.URL.Query().Get("file")
	if fileName == "" {
		WriteResponse(w, http.StatusBadRequest, "file is required")
		return
	}
	if err := validateFileName(fileName); err != nil {
		WriteResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	var item FileItem

	wantObject, _, err := resolveStoreSelection(storeType, false)
	if err != nil {
		WriteResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	if wantObject {
		objectStores, _ := collectStores(workspace)
		objectStore, err := selectObjectStore(objectStores)
		if err != nil {
			WriteResponse(w, http.StatusBadRequest, err.Error())
			return
		}
		item, err = svc.getObjectStoreMetadata(r, objectStore, fileName)
		if err != nil {
			WriteResponse(w, http.StatusNotFound, err.Error())
			return
		}
	} else {
		_, blockStores := collectStores(workspace)
		blockStore, err := selectBlockStore(blockStores)
		if err != nil {
			WriteResponse(w, http.StatusBadRequest, err.Error())
			return
		}
		item, err = svc.getBlockStoreMetadata(ctx, workspaceID, blockStore, fileName)
		if err != nil {
			WriteResponse(w, http.StatusNotFound, err.Error())
			return
		}
	}

	WriteResponse(w, http.StatusOK, FileMetadataResponse{
		Workspace: workspaceID,
		Item:      item,
	})
}
