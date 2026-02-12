package services

import (
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	ws_manager "github.com/EO-DataHub/eodhp-workspace-manager/models"
	"github.com/EO-DataHub/eodhp-workspace-services/api/middleware"
	"github.com/EO-DataHub/eodhp-workspace-services/db"
	"github.com/EO-DataHub/eodhp-workspace-services/internal/appconfig"
	"github.com/EO-DataHub/eodhp-workspace-services/internal/authn"
	awsclient "github.com/EO-DataHub/eodhp-workspace-services/internal/aws"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
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
		return false, false, fmt.Errorf(invalidStoreType)
	case storeTypeObject:
		return true, false, nil
	case storeTypeBlock:
		return false, true, nil
	default:
		return false, false, fmt.Errorf(invalidStoreType)
	}
}

// ListFilesService lists files from object and/or block stores.
func (svc *FileService) ListFilesService(w http.ResponseWriter, r *http.Request) {
	workspaceID, workspace, ok := svc.resolveAuthorizedWorkspace(w, r)
	if !ok {
		return
	}

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
		blkItems, err := svc.listBlockStoreItems(blockStores)
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
		uploaded, err := svc.uploadBlockStoreFiles(blockStore, files)
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
		deleted, failed, err = svc.deleteBlockStoreFiles(blockStore, []string{fileName})
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
		item, err = svc.getBlockStoreMetadata(blockStore, fileName)
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

func (svc *FileService) listObjectStoreItems(r *http.Request, stores []ws_manager.ObjectStore) ([]FileItem, error) {
	if len(stores) == 0 {
		return nil, fmt.Errorf("no object store configured")
	}
	store, err := selectObjectStore(stores)
	if err != nil {
		return nil, err
	}
	if store.Bucket == "" || store.Prefix == "" {
		return nil, fmt.Errorf("object store not provisioned")
	}

	s3Client, err := svc.newS3Client(r)
	if err != nil {
		return nil, err
	}

	prefix, err := safeS3Prefix(store.Prefix, "")
	if err != nil {
		return nil, err
	}

	items, err := listS3Objects(r.Context(), s3Client, store, prefix, svc.responseTimeFormat())
	if err != nil {
		return nil, err
	}

	return items, nil
}

func (svc *FileService) listBlockStoreItems(stores []ws_manager.BlockStore) ([]FileItem, error) {
	if len(stores) == 0 {
		return nil, fmt.Errorf("no block store configured")
	}
	store, err := selectBlockStore(stores)
	if err != nil {
		return nil, err
	}
	if store.MountPoint == "" {
		return nil, fmt.Errorf("block store not provisioned")
	}

	root, err := safeBlockPath(store.MountPoint, "")
	if err != nil {
		return nil, err
	}
	return listBlockFiles(root, svc.responseTimeFormat())
}

func (svc *FileService) uploadObjectStoreFiles(r *http.Request, store ws_manager.ObjectStore, files []*multipart.FileHeader) ([]FileItem, error) {
	if store.Bucket == "" || store.Prefix == "" {
		return nil, fmt.Errorf("object store not provisioned")
	}

	s3Client, err := svc.newS3Client(r)
	if err != nil {
		return nil, err
	}

	var items []FileItem
	for _, fh := range files {
		if fh == nil || fh.Filename == "" {
			continue
		}
		if err := validateFileName(fh.Filename); err != nil {
			return nil, err
		}

		src, err := fh.Open()
		if err != nil {
			return nil, err
		}

		key, err := safeS3Key(store.Prefix, fh.Filename)
		if err != nil {
			_ = src.Close()
			return nil, err
		}

		_, err = s3Client.PutObject(r.Context(), &s3.PutObjectInput{
			Bucket:      aws.String(store.Bucket),
			Key:         aws.String(key),
			Body:        src,
			ContentType: aws.String(fh.Header.Get("Content-Type")),
		})
		if closeErr := src.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
		if err != nil {
			return nil, err
		}

		items = append(items, FileItem{
			StoreType: storeTypeObject,
			FileName:  relativeS3Path(store.Prefix, key),
			Size:      fh.Size,
		})
	}

	return items, nil
}

func (svc *FileService) uploadBlockStoreFiles(store ws_manager.BlockStore, files []*multipart.FileHeader) ([]FileItem, error) {
	if store.MountPoint == "" {
		return nil, fmt.Errorf("block store not provisioned")
	}

	root, err := safeBlockPath(store.MountPoint, "")
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, err
	}

	var items []FileItem
	for _, fh := range files {
		if fh == nil || fh.Filename == "" {
			continue
		}
		if err := validateFileName(fh.Filename); err != nil {
			return nil, err
		}

		src, err := fh.Open()
		if err != nil {
			return nil, err
		}

		dstPath, err := safeBlockPath(root, fh.Filename)
		if err != nil {
			_ = src.Close()
			return nil, err
		}

		dst, err := os.Create(dstPath)
		if err != nil {
			_ = src.Close()
			return nil, err
		}

		written, err := io.Copy(dst, src)
		if closeErr := dst.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
		if closeErr := src.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
		if err != nil {
			return nil, err
		}

		info, err := os.Stat(dstPath)
		if err != nil {
			return nil, err
		}

		lastModified := ""
		if !info.ModTime().IsZero() {
			lastModified = info.ModTime().UTC().Format(svc.responseTimeFormat())
		}

		items = append(items, FileItem{
			StoreType:    storeTypeBlock,
			FileName:     fh.Filename,
			Size:         written,
			LastModified: lastModified,
		})
	}

	return items, nil
}

func (svc *FileService) deleteObjectStoreFiles(r *http.Request, store ws_manager.ObjectStore, paths []string) ([]string, []FileFail, error) {
	if store.Bucket == "" || store.Prefix == "" {
		return nil, nil, fmt.Errorf("object store not provisioned")
	}

	s3Client, err := svc.newS3Client(r)
	if err != nil {
		return nil, nil, err
	}

	var deleted []string
	var failed []FileFail
	for _, p := range paths {
		key, err := safeS3Key(store.Prefix, p)
		if err != nil {
			failed = append(failed, FileFail{FileName: p, Error: err.Error()})
			continue
		}

		_, err = s3Client.DeleteObject(r.Context(), &s3.DeleteObjectInput{
			Bucket: aws.String(store.Bucket),
			Key:    aws.String(key),
		})
		if err != nil {
			failed = append(failed, FileFail{FileName: p, Error: err.Error()})
			continue
		}
		deleted = append(deleted, p)
	}

	return deleted, failed, nil
}

func (svc *FileService) deleteBlockStoreFiles(store ws_manager.BlockStore, paths []string) ([]string, []FileFail, error) {
	if store.MountPoint == "" {
		return nil, nil, fmt.Errorf("block store not provisioned")
	}

	root, err := safeBlockPath(store.MountPoint, "")
	if err != nil {
		return nil, nil, err
	}

	var deleted []string
	var failed []FileFail
	for _, p := range paths {
		fullPath, err := safeBlockPath(root, p)
		if err != nil {
			failed = append(failed, FileFail{FileName: p, Error: err.Error()})
			continue
		}
		info, err := os.Stat(fullPath)
		if err != nil {
			failed = append(failed, FileFail{FileName: p, Error: err.Error()})
			continue
		}
		if info.IsDir() {
			failed = append(failed, FileFail{FileName: p, Error: "file is a directory"})
			continue
		}
		if err := os.Remove(fullPath); err != nil {
			failed = append(failed, FileFail{FileName: p, Error: err.Error()})
			continue
		}
		deleted = append(deleted, p)
	}

	return deleted, failed, nil
}

func (svc *FileService) getObjectStoreMetadata(r *http.Request, store ws_manager.ObjectStore, pathParam string) (FileItem, error) {
	if store.Bucket == "" || store.Prefix == "" {
		return FileItem{}, fmt.Errorf("object store not provisioned")
	}

	s3Client, err := svc.newS3Client(r)
	if err != nil {
		return FileItem{}, err
	}

	key, err := safeS3Key(store.Prefix, pathParam)
	if err != nil {
		return FileItem{}, err
	}

	resp, err := s3Client.HeadObject(r.Context(), &s3.HeadObjectInput{
		Bucket: aws.String(store.Bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return FileItem{}, err
	}

	item := FileItem{
		StoreType: storeTypeObject,
		FileName:  relativeS3Path(store.Prefix, key),
		Size:      aws.ToInt64(resp.ContentLength),
	}
	if resp.LastModified != nil {
		item.LastModified = resp.LastModified.UTC().Format(svc.responseTimeFormat())
	}
	if resp.ETag != nil {
		item.ETag = strings.Trim(*resp.ETag, "\"")
	}
	return item, nil
}

func (svc *FileService) getBlockStoreMetadata(store ws_manager.BlockStore, pathParam string) (FileItem, error) {
	if store.MountPoint == "" {
		return FileItem{}, fmt.Errorf("block store not provisioned")
	}

	root, err := safeBlockPath(store.MountPoint, "")
	if err != nil {
		return FileItem{}, err
	}
	fullPath, err := safeBlockPath(root, pathParam)
	if err != nil {
		return FileItem{}, err
	}

	info, err := os.Stat(fullPath)
	if err != nil {
		return FileItem{}, err
	}
	item := FileItem{
		StoreType: storeTypeBlock,
		FileName:  filepath.Base(fullPath),
		Size:      info.Size(),
	}
	item.LastModified = info.ModTime().UTC().Format(svc.responseTimeFormat())
	return item, nil
}

func (svc *FileService) newS3Client(r *http.Request) (*s3.Client, error) {
	creds, err := svc.getS3Credentials(r)
	if err != nil {
		return nil, err
	}
	cfg, err := config.LoadDefaultConfig(r.Context(),
		config.WithRegion(svc.Config.AWS.Region),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			creds.AccessKeyId,
			creds.SecretAccessKey,
			creds.SessionToken,
		)),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to configure S3 client: %w", err)
	}
	return awsclient.NewS3ClientWithEndpoint(cfg, svc.Config.AWS.S3.Endpoint, svc.Config.AWS.S3.ForcePathStyle), nil
}

func (svc *FileService) getS3Credentials(r *http.Request) (awsclient.S3Credentials, error) {
	// Local/dev override: use static S3 keys when provided instead of STS.
	if svc.Config.AWS.S3.AccessKey != "" && svc.Config.AWS.S3.SecretKey != "" {
		return awsclient.S3Credentials{
			AccessKeyId:     svc.Config.AWS.S3.AccessKey,
			SecretAccessKey: svc.Config.AWS.S3.SecretKey,
			SessionToken:    "",
			Expiration:      "",
		}, nil
	}

	token := extractBearerToken(r.Header.Get("Authorization"))
	if token == "" {
		return awsclient.S3Credentials{}, fmt.Errorf("authorization header missing")
	}

	roleARN := strings.TrimSpace(svc.Config.AWS.S3.RoleArn)
	if roleARN == "" {
		return awsclient.S3Credentials{}, fmt.Errorf("missing AWS role ARN for S3 credentials")
	}

	stsClient := svc.STS
	if stsClient == nil {
		cfg, err := config.LoadDefaultConfig(r.Context(), config.WithRegion(svc.Config.AWS.Region))
		if err != nil {
			return awsclient.S3Credentials{}, fmt.Errorf("failed to load AWS config: %w", err)
		}
		stsClient = sts.NewFromConfig(cfg)
	}
	out, err := stsClient.AssumeRoleWithWebIdentity(r.Context(), &sts.AssumeRoleWithWebIdentityInput{
		RoleArn:          aws.String(roleARN),
		RoleSessionName:  aws.String("workspace-services"),
		WebIdentityToken: aws.String(token),
	})
	if err != nil {
		return awsclient.S3Credentials{}, err
	}
	if out.Credentials == nil {
		return awsclient.S3Credentials{}, fmt.Errorf("missing credentials from STS response")
	}
	resp := out

	if resp.Credentials.AccessKeyId == nil || resp.Credentials.SecretAccessKey == nil || resp.Credentials.SessionToken == nil || resp.Credentials.Expiration == nil {
		return awsclient.S3Credentials{}, fmt.Errorf("invalid credentials returned by STS")
	}

	return awsclient.S3Credentials{
		AccessKeyId:     *resp.Credentials.AccessKeyId,
		SecretAccessKey: *resp.Credentials.SecretAccessKey,
		SessionToken:    *resp.Credentials.SessionToken,
		Expiration:      resp.Credentials.Expiration.UTC().Format(svc.responseTimeFormat()),
	}, nil
}
