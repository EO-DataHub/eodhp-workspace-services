package services

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

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
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/gorilla/mux"
	"github.com/rs/zerolog"
)

const (
	storeTypeObject = "object"
	storeTypeBlock  = "block"
	timeFormat      = "2006-01-02T15:04:05Z"
	maxFormMemory   = int64(32 << 20) // 32MB
	downloadExpiry  = 10 * time.Minute
)

// STSClient defines the minimal interface needed for STS AssumeRoleWithWebIdentity.
type STSClient interface {
	AssumeRoleWithWebIdentity(ctx context.Context,
		params *sts.AssumeRoleWithWebIdentityInput, optFns ...func(*sts.Options)) (
		*sts.AssumeRoleWithWebIdentityOutput, error)
}

type S3Credentials struct {
	AccessKeyId     string
	SecretAccessKey string
	SessionToken    string
	Expiration      string
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
	DownloadURL  string `json:"downloadUrl,omitempty"`
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

// ListFilesService lists files from object and/or block stores.
func (svc *FileService) ListFilesService(w http.ResponseWriter, r *http.Request) {
	logger := zerolog.Ctx(r.Context())

	workspaceID := mux.Vars(r)["workspace-id"]
	claims, ok := r.Context().Value(middleware.ClaimsKey).(authn.Claims)
	if !ok {
		logger.Warn().Msg("Unauthorized request: missing claims")
		WriteResponse(w, http.StatusUnauthorized, "missing claims")
		return
	}

	authorized, err := isUserWorkspaceAuthorized(svc.DB, svc.KC, claims, workspaceID, false)
	if err != nil {
		logger.Error().Err(err).Str("workspace_id", workspaceID).Msg("Failed to authorize workspace")
		WriteResponse(w, http.StatusInternalServerError, "authorization failed")
		return
	}
	if !authorized {
		logger.Warn().Str("workspace_id", workspaceID).Msg("Access denied")
		WriteResponse(w, http.StatusForbidden, "access denied")
		return
	}

	workspace, err := svc.DB.GetWorkspace(workspaceID)
	if err != nil {
		logger.Error().Err(err).Str("workspace_id", workspaceID).Msg("Workspace not found")
		WriteResponse(w, http.StatusNotFound, "workspace not found")
		return
	}

	storeType := strings.ToLower(r.URL.Query().Get("store"))

	objectStores, blockStores := collectStores(workspace)
	var items []FileItem

	switch storeType {
	case "":
		objItems, err := svc.listObjectStoreItems(r, objectStores)
		if err != nil {
			WriteResponse(w, http.StatusBadRequest, err.Error())
			return
		}
		blkItems, err := svc.listBlockStoreItems(blockStores)
		if err != nil {
			WriteResponse(w, http.StatusBadRequest, err.Error())
			return
		}
		items = append(items, objItems...)
		items = append(items, blkItems...)
	case storeTypeObject:
		objItems, err := svc.listObjectStoreItems(r, objectStores)
		if err != nil {
			WriteResponse(w, http.StatusBadRequest, err.Error())
			return
		}
		items = objItems
	case storeTypeBlock:
		blkItems, err := svc.listBlockStoreItems(blockStores)
		if err != nil {
			WriteResponse(w, http.StatusBadRequest, err.Error())
			return
		}
		items = blkItems
	default:
		WriteResponse(w, http.StatusBadRequest, "invalid store type")
		return
	}

	items = svc.withDownloadURLs(r, workspaceID, items)

	WriteResponse(w, http.StatusOK, FileListResponse{
		Workspace: workspaceID,
		Items:     items,
	})
}

// UploadFilesService uploads files to a single store.
func (svc *FileService) UploadFilesService(w http.ResponseWriter, r *http.Request, storeType string) {
	logger := zerolog.Ctx(r.Context())

	workspaceID := mux.Vars(r)["workspace-id"]
	claims, ok := r.Context().Value(middleware.ClaimsKey).(authn.Claims)
	if !ok {
		logger.Warn().Msg("Unauthorized request: missing claims")
		WriteResponse(w, http.StatusUnauthorized, "missing claims")
		return
	}

	authorized, err := isUserWorkspaceAuthorized(svc.DB, svc.KC, claims, workspaceID, false)
	if err != nil {
		logger.Error().Err(err).Str("workspace_id", workspaceID).Msg("Failed to authorize workspace")
		WriteResponse(w, http.StatusInternalServerError, "authorization failed")
		return
	}
	if !authorized {
		logger.Warn().Str("workspace_id", workspaceID).Msg("Access denied")
		WriteResponse(w, http.StatusForbidden, "access denied")
		return
	}

	workspace, err := svc.DB.GetWorkspace(workspaceID)
	if err != nil {
		logger.Error().Err(err).Str("workspace_id", workspaceID).Msg("Workspace not found")
		WriteResponse(w, http.StatusNotFound, "workspace not found")
		return
	}

	if err := r.ParseMultipartForm(maxFormMemory); err != nil {
		WriteResponse(w, http.StatusBadRequest, "invalid multipart form data")
		return
	}
	files := collectMultipartFiles(r.MultipartForm)
	if len(files) == 0 {
		WriteResponse(w, http.StatusBadRequest, "no files provided")
		return
	}

	var items []FileItem
	switch storeType {
	case storeTypeObject:
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
	case storeTypeBlock:
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
	default:
		WriteResponse(w, http.StatusBadRequest, "invalid store type")
		return
	}

	items = svc.withDownloadURLs(r, workspaceID, items)

	WriteResponse(w, http.StatusCreated, FileUploadResponse{
		Workspace: workspaceID,
		Items:     items,
	})
}

// DeleteFilesService deletes files from a single store.
func (svc *FileService) DeleteFilesService(w http.ResponseWriter, r *http.Request, storeType string) {
	logger := zerolog.Ctx(r.Context())

	workspaceID := mux.Vars(r)["workspace-id"]
	claims, ok := r.Context().Value(middleware.ClaimsKey).(authn.Claims)
	if !ok {
		logger.Warn().Msg("Unauthorized request: missing claims")
		WriteResponse(w, http.StatusUnauthorized, "missing claims")
		return
	}

	authorized, err := isUserWorkspaceAuthorized(svc.DB, svc.KC, claims, workspaceID, false)
	if err != nil {
		logger.Error().Err(err).Str("workspace_id", workspaceID).Msg("Failed to authorize workspace")
		WriteResponse(w, http.StatusInternalServerError, "authorization failed")
		return
	}
	if !authorized {
		logger.Warn().Str("workspace_id", workspaceID).Msg("Access denied")
		WriteResponse(w, http.StatusForbidden, "access denied")
		return
	}

	workspace, err := svc.DB.GetWorkspace(workspaceID)
	if err != nil {
		logger.Error().Err(err).Str("workspace_id", workspaceID).Msg("Workspace not found")
		WriteResponse(w, http.StatusNotFound, "workspace not found")
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

	switch storeType {
	case storeTypeObject:
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
	case storeTypeBlock:
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
	default:
		WriteResponse(w, http.StatusBadRequest, "invalid store type")
		return
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
	logger := zerolog.Ctx(r.Context())

	workspaceID := mux.Vars(r)["workspace-id"]
	claims, ok := r.Context().Value(middleware.ClaimsKey).(authn.Claims)
	if !ok {
		logger.Warn().Msg("Unauthorized request: missing claims")
		WriteResponse(w, http.StatusUnauthorized, "missing claims")
		return
	}

	authorized, err := isUserWorkspaceAuthorized(svc.DB, svc.KC, claims, workspaceID, false)
	if err != nil {
		logger.Error().Err(err).Str("workspace_id", workspaceID).Msg("Failed to authorize workspace")
		WriteResponse(w, http.StatusInternalServerError, "authorization failed")
		return
	}
	if !authorized {
		logger.Warn().Str("workspace_id", workspaceID).Msg("Access denied")
		WriteResponse(w, http.StatusForbidden, "access denied")
		return
	}

	workspace, err := svc.DB.GetWorkspace(workspaceID)
	if err != nil {
		logger.Error().Err(err).Str("workspace_id", workspaceID).Msg("Workspace not found")
		WriteResponse(w, http.StatusNotFound, "workspace not found")
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

	switch storeType {
	case storeTypeObject:
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
		downloadURL, err := svc.presignObjectDownload(r, objectStore, item.FileName)
		if err != nil {
			WriteResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
		item.DownloadURL = downloadURL
	case storeTypeBlock:
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
		item.DownloadURL = svc.blockDownloadURL(r, workspaceID, item.FileName)
		if item.DownloadURL == "" {
			WriteResponse(w, http.StatusInternalServerError, "failed to generate download url")
			return
		}
	default:
		WriteResponse(w, http.StatusBadRequest, "invalid store type")
		return
	}

	WriteResponse(w, http.StatusOK, FileMetadataResponse{
		Workspace: workspaceID,
		Item:      item,
	})
}

// DownloadBlockFileService streams a single file from the block store.
func (svc *FileService) DownloadBlockFileService(w http.ResponseWriter, r *http.Request) {
	logger := zerolog.Ctx(r.Context())

	workspaceID := mux.Vars(r)["workspace-id"]

	fileName := r.URL.Query().Get("file")
	if fileName == "" {
		WriteResponse(w, http.StatusBadRequest, "file is required")
		return
	}
	if err := validateFileName(fileName); err != nil {
		WriteResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	exp := r.URL.Query().Get("exp")
	sig := r.URL.Query().Get("sig")
	if exp == "" || sig == "" {
		WriteResponse(w, http.StatusUnauthorized, "signed url required")
		return
	}
	if err := svc.validateBlockDownloadSignature(workspaceID, fileName, exp, sig); err != nil {
		logger.Warn().Err(err).Str("workspace_id", workspaceID).Msg("Invalid download signature")
		WriteResponse(w, http.StatusUnauthorized, "invalid or expired download url")
		return
	}

	workspace, err := svc.DB.GetWorkspace(workspaceID)
	if err != nil {
		logger.Error().Err(err).Str("workspace_id", workspaceID).Msg("Workspace not found")
		WriteResponse(w, http.StatusNotFound, "workspace not found")
		return
	}

	_, blockStores := collectStores(workspace)
	blockStore, err := selectBlockStore(blockStores)
	if err != nil {
		WriteResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	if blockStore.MountPoint == "" {
		WriteResponse(w, http.StatusBadRequest, "block store not provisioned")
		return
	}

	fullPath, err := safeBlockPath(blockStore.MountPoint, fileName)
	if err != nil {
		WriteResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	info, err := os.Stat(fullPath)
	if err != nil {
		WriteResponse(w, http.StatusNotFound, err.Error())
		return
	}
	if info.IsDir() {
		WriteResponse(w, http.StatusBadRequest, "file is a directory")
		return
	}

	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", fileName))
	http.ServeFile(w, r, fullPath)
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

	items, err := listS3Objects(r.Context(), s3Client, store, prefix)
	if err != nil {
		return nil, err
	}

	presigner, err := svc.newS3PresignClient(r)
	if err != nil {
		return nil, err
	}
	for i := range items {
		if items[i].FileName == "" {
			continue
		}
		url, err := presignObjectDownloadWithPresigner(r.Context(), presigner, store, items[i].FileName)
		if err != nil {
			return nil, err
		}
		items[i].DownloadURL = url
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
	return listBlockFiles(root)
}

func (svc *FileService) uploadObjectStoreFiles(r *http.Request, store ws_manager.ObjectStore, files []*multipart.FileHeader) ([]FileItem, error) {
	if store.Bucket == "" || store.Prefix == "" {
		return nil, fmt.Errorf("object store not provisioned")
	}

	s3Client, err := svc.newS3Client(r)
	if err != nil {
		return nil, err
	}
	presigner, err := svc.newS3PresignClient(r)
	if err != nil {
		return nil, err
	}

	var items []FileItem
	for _, fh := range files {
		src, err := fh.Open()
		if err != nil {
			return nil, err
		}

		if err := validateFileName(fh.Filename); err != nil {
			src.Close()
			return nil, err
		}
		key, err := safeS3Key(store.Prefix, fh.Filename)
		if err != nil {
			src.Close()
			return nil, err
		}

		input := &s3.PutObjectInput{
			Bucket: aws.String(store.Bucket),
			Key:    aws.String(key),
			Body:   src,
		}
		if fh.Size > 0 {
			input.ContentLength = aws.Int64(fh.Size)
		}
		if ct := fh.Header.Get("Content-Type"); ct != "" {
			input.ContentType = aws.String(ct)
		}

		_, err = s3Client.PutObject(r.Context(), input)
		src.Close()
		if err != nil {
			return nil, err
		}

		downloadURL, err := presignObjectDownloadWithPresigner(r.Context(), presigner, store, fh.Filename)
		if err != nil {
			return nil, err
		}

		items = append(items, FileItem{
			StoreType:   storeTypeObject,
			FileName:    relativeS3Path(store.Prefix, key),
			DownloadURL: downloadURL,
			Size:        fh.Size,
		})
	}

	return items, nil
}

func (svc *FileService) uploadBlockStoreFiles(store ws_manager.BlockStore, files []*multipart.FileHeader) ([]FileItem, error) {
	if store.MountPoint == "" {
		return nil, fmt.Errorf("block store not provisioned")
	}
	var items []FileItem
	for _, fh := range files {
		if err := validateFileName(fh.Filename); err != nil {
			return nil, err
		}
		relPath := filepath.Join(fh.Filename)
		fullPath, err := safeBlockPath(store.MountPoint, relPath)
		if err != nil {
			return nil, err
		}
		if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
			return nil, err
		}
		src, err := fh.Open()
		if err != nil {
			return nil, err
		}

		dst, err := os.Create(fullPath)
		if err != nil {
			src.Close()
			return nil, err
		}
		size, err := io.Copy(dst, src)
		src.Close()
		closeErr := dst.Close()
		if err != nil {
			return nil, err
		}
		if closeErr != nil {
			return nil, closeErr
		}

		items = append(items, FileItem{
			StoreType: storeTypeBlock,
			FileName:  filepath.Base(fullPath),
			Size:      size,
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

	var objects []s3types.ObjectIdentifier
	var deleted []string
	var failed []FileFail

	for _, p := range paths {
		if err := validateFileName(p); err != nil {
			failed = append(failed, FileFail{FileName: p, Error: err.Error()})
			continue
		}
		key, err := safeS3Key(store.Prefix, p)
		if err != nil {
			failed = append(failed, FileFail{FileName: p, Error: err.Error()})
			continue
		}
		objects = append(objects, s3types.ObjectIdentifier{Key: aws.String(key)})
	}

	if len(objects) == 0 {
		return deleted, failed, nil
	}

	output, err := s3Client.DeleteObjects(r.Context(), &s3.DeleteObjectsInput{
		Bucket: aws.String(store.Bucket),
		Delete: &s3types.Delete{
			Objects: objects,
			Quiet:   aws.Bool(false),
		},
	})
	if err != nil {
		return nil, nil, err
	}

	for _, d := range output.Deleted {
		deleted = append(deleted, relativeS3Path(store.Prefix, aws.ToString(d.Key)))
	}

	for _, delErr := range output.Errors {
		failed = append(failed, FileFail{
			FileName: relativeS3Path(store.Prefix, aws.ToString(delErr.Key)),
			Error:    aws.ToString(delErr.Message),
		})
	}

	return deleted, failed, nil
}

func (svc *FileService) deleteBlockStoreFiles(store ws_manager.BlockStore, paths []string) ([]string, []FileFail, error) {
	if store.MountPoint == "" {
		return nil, nil, fmt.Errorf("block store not provisioned")
	}

	var deleted []string
	var failed []FileFail

	for _, p := range paths {
		if err := validateFileName(p); err != nil {
			failed = append(failed, FileFail{FileName: p, Error: err.Error()})
			continue
		}
		fullPath, err := safeBlockPath(store.MountPoint, p)
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
		item.LastModified = resp.LastModified.UTC().Format(timeFormat)
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
	fullPath, err := safeBlockPath(store.MountPoint, pathParam)
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
	item.LastModified = info.ModTime().UTC().Format(timeFormat)
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

func (svc *FileService) newS3PresignClient(r *http.Request) (*s3.PresignClient, error) {
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
		return nil, fmt.Errorf("failed to configure S3 presign client: %w", err)
	}
	endpoint := strings.TrimSpace(svc.Config.AWS.S3.PublicEndpoint)
	if endpoint == "" {
		endpoint = svc.Config.AWS.S3.Endpoint
	}
	client := awsclient.NewS3ClientWithEndpoint(cfg, endpoint, svc.Config.AWS.S3.ForcePathStyle)
	return s3.NewPresignClient(client), nil
}

func (svc *FileService) presignObjectDownload(r *http.Request, store ws_manager.ObjectStore, fileName string) (string, error) {
	if store.Bucket == "" || store.Prefix == "" {
		return "", fmt.Errorf("object store not provisioned")
	}
	presigner, err := svc.newS3PresignClient(r)
	if err != nil {
		return "", err
	}
	return presignObjectDownloadWithPresigner(r.Context(), presigner, store, fileName)
}

func presignObjectDownloadWithPresigner(ctx context.Context, presigner *s3.PresignClient, store ws_manager.ObjectStore, fileName string) (string, error) {
	key, err := safeS3Key(store.Prefix, fileName)
	if err != nil {
		return "", err
	}

	req, err := presigner.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(store.Bucket),
		Key:    aws.String(key),
	}, s3.WithPresignExpires(downloadExpiry))
	if err != nil {
		return "", err
	}
	return req.URL, nil
}

func (svc *FileService) withDownloadURLs(r *http.Request, workspaceID string, items []FileItem) []FileItem {
	for i := range items {
		if items[i].StoreType != storeTypeBlock {
			continue
		}
		if items[i].FileName == "" {
			continue
		}
		if err := validateFileName(items[i].FileName); err != nil {
			continue
		}
		items[i].DownloadURL = svc.blockDownloadURL(r, workspaceID, items[i].FileName)
	}
	return items
}

func (svc *FileService) blockDownloadURL(r *http.Request, workspaceID, fileName string) string {
	if workspaceID == "" || fileName == "" {
		return ""
	}
	secret := strings.TrimSpace(svc.Config.Download.SigningSecret)
	if secret == "" {
		return ""
	}
	exp := time.Now().Add(downloadExpiry).Unix()
	sig := signBlockDownload(secret, workspaceID, fileName, exp)

	base := svc.baseURL(r)
	if base == "" {
		return ""
	}
	u, err := url.Parse(base)
	if err != nil {
		return ""
	}
	apiPath := path.Join(svc.Config.BasePath, "workspaces", workspaceID, "files", storeTypeBlock, "download")
	u.Path = path.Join(u.Path, apiPath)
	q := u.Query()
	q.Set("file", fileName)
	q.Set("exp", strconv.FormatInt(exp, 10))
	q.Set("sig", sig)
	u.RawQuery = q.Encode()
	return u.String()
}

func (svc *FileService) validateBlockDownloadSignature(workspaceID, fileName, expStr, sig string) error {
	secret := strings.TrimSpace(svc.Config.Download.SigningSecret)
	if secret == "" {
		return fmt.Errorf("download signing secret is not configured")
	}
	exp, err := strconv.ParseInt(expStr, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid expiry")
	}
	if time.Now().Unix() > exp {
		return fmt.Errorf("expired")
	}

	expected := signBlockDownload(secret, workspaceID, fileName, exp)
	if !hmac.Equal([]byte(sig), []byte(expected)) {
		return fmt.Errorf("signature mismatch")
	}
	return nil
}

func signBlockDownload(secret, workspaceID, fileName string, exp int64) string {
	mac := hmac.New(sha256.New, []byte(secret))
	fmt.Fprintf(mac, "%s:%s:%d", workspaceID, fileName, exp)
	return hex.EncodeToString(mac.Sum(nil))
}

func (svc *FileService) baseURL(r *http.Request) string {
	host := strings.TrimSpace(svc.Config.Host)
	if host == "" {
		host = r.Host
	}
	if host == "" {
		return ""
	}
	if strings.HasPrefix(host, "http://") || strings.HasPrefix(host, "https://") {
		return host
	}
	scheme := "http"
	if proto := r.Header.Get("X-Forwarded-Proto"); proto != "" {
		scheme = proto
	} else if r.TLS != nil {
		scheme = "https"
	}
	return fmt.Sprintf("%s://%s", scheme, host)
}

func (svc *FileService) getS3Credentials(r *http.Request) (S3Credentials, error) {
	if svc.Config.AWS.S3.AccessKey != "" && svc.Config.AWS.S3.SecretKey != "" {
		return S3Credentials{
			AccessKeyId:     svc.Config.AWS.S3.AccessKey,
			SecretAccessKey: svc.Config.AWS.S3.SecretKey,
			SessionToken:    "",
			Expiration:      "",
		}, nil
	}

	vars := mux.Vars(r)
	workspaceID := vars["workspace-id"]

	logger := zerolog.Ctx(r.Context()).With().Str("workspace", workspaceID).
		Str("role arn", svc.Config.AWS.S3.RoleArn).Logger()

	token, ok := r.Context().Value(middleware.TokenKey).(string)
	if !ok {
		err := fmt.Errorf("invalid token")
		logger.Error().Msg(err.Error())
		return S3Credentials{}, err
	}

	claims, ok := r.Context().Value(middleware.ClaimsKey).(authn.Claims)
	if !ok {
		err := fmt.Errorf("invalid claims")
		logger.Error().Msg(err.Error())
		return S3Credentials{}, err
	}

	if claims.Workspace != workspaceID {
		workspaceToken, err := svc.KC.ExchangeToken(token, fmt.Sprintf("workspace:%s", workspaceID))
		if err != nil {
			logger.Error().Err(err).Msg("Failed to exchange token")
			return S3Credentials{}, err
		}
		token = workspaceToken.Access
	}

	resp, err := svc.STS.AssumeRoleWithWebIdentity(r.Context(), &sts.AssumeRoleWithWebIdentityInput{
		RoleArn:          aws.String(svc.Config.AWS.S3.RoleArn),
		WebIdentityToken: aws.String(token),
		RoleSessionName:  aws.String(fmt.Sprintf("%s-%s", workspaceID, claims.Username)),
	})
	if err != nil {
		logger.Err(err).Msg("Failed to retrieve S3 credentials")
		return S3Credentials{}, err
	}

	return S3Credentials{
		AccessKeyId:     *resp.Credentials.AccessKeyId,
		SecretAccessKey: *resp.Credentials.SecretAccessKey,
		SessionToken:    *resp.Credentials.SessionToken,
		Expiration:      resp.Credentials.Expiration.UTC().Format(timeFormat),
	}, nil
}

// Helper functions

func collectStores(workspace *ws_manager.WorkspaceSettings) ([]ws_manager.ObjectStore, []ws_manager.BlockStore) {
	var objectStores []ws_manager.ObjectStore
	var blockStores []ws_manager.BlockStore

	if workspace.Stores == nil {
		return objectStores, blockStores
	}
	for _, store := range *workspace.Stores {
		objectStores = append(objectStores, store.Object...)
		blockStores = append(blockStores, store.Block...)
	}
	return objectStores, blockStores
}

func selectObjectStore(stores []ws_manager.ObjectStore) (ws_manager.ObjectStore, error) {
	if len(stores) == 0 {
		return ws_manager.ObjectStore{}, fmt.Errorf("no object store configured")
	}
	return stores[0], nil
}

func selectBlockStore(stores []ws_manager.BlockStore) (ws_manager.BlockStore, error) {
	if len(stores) == 0 {
		return ws_manager.BlockStore{}, fmt.Errorf("no block store configured")
	}
	return stores[0], nil
}

func collectMultipartFiles(form *multipart.Form) []*multipart.FileHeader {
	if form == nil {
		return nil
	}
	var files []*multipart.FileHeader
	for _, fhs := range form.File {
		files = append(files, fhs...)
	}
	return files
}

func safeS3Prefix(prefix, rel string) (string, error) {
	if prefix == "" {
		return "", fmt.Errorf("invalid object store prefix")
	}
	cleanPrefix := strings.TrimSuffix(prefix, "/")
	if rel == "" {
		return cleanPrefix + "/", nil
	}
	rel = path.Clean("/" + rel)
	if strings.HasPrefix(rel, "/..") {
		return "", fmt.Errorf("invalid path")
	}
	rel = strings.TrimPrefix(rel, "/")
	return cleanPrefix + "/" + rel, nil
}

func safeS3Key(prefix, rel string) (string, error) {
	if prefix == "" {
		return "", fmt.Errorf("invalid object store prefix")
	}
	cleanPrefix := strings.TrimSuffix(prefix, "/")
	rel = path.Clean("/" + rel)
	if strings.HasPrefix(rel, "/..") {
		return "", fmt.Errorf("invalid path")
	}
	rel = strings.TrimPrefix(rel, "/")
	if rel == "." || rel == "" {
		return cleanPrefix, nil
	}
	return cleanPrefix + "/" + rel, nil
}

func relativeS3Path(prefix, key string) string {
	cleanPrefix := strings.TrimSuffix(prefix, "/") + "/"
	if strings.HasPrefix(key, cleanPrefix) {
		return strings.TrimPrefix(key, cleanPrefix)
	}
	return key
}

func safeBlockPath(root, rel string) (string, error) {
	if root == "" {
		return "", fmt.Errorf("invalid mount point")
	}
	root = filepath.Clean(root)
	if !filepath.IsAbs(root) {
		absRoot, err := filepath.Abs(root)
		if err == nil {
			root = absRoot
		}
	}

	rel = filepath.Clean(rel)
	if rel == "." || rel == string(filepath.Separator) {
		rel = ""
	}
	if filepath.IsAbs(rel) || strings.HasPrefix(rel, "..") {
		return "", fmt.Errorf("invalid path")
	}
	full := filepath.Join(root, rel)
	relToRoot, err := filepath.Rel(root, full)
	if err != nil || strings.HasPrefix(relToRoot, "..") {
		return "", fmt.Errorf("invalid path")
	}
	return full, nil
}

func validateFileName(name string) error {
	name = strings.TrimSpace(name)
	if name == "" || name == "." {
		return fmt.Errorf("file name is required")
	}
	if strings.Contains(name, "/") || strings.Contains(name, "\\") {
		return fmt.Errorf("file name must not contain path separators")
	}
	return nil
}

func listS3Objects(ctx context.Context, client *s3.Client, store ws_manager.ObjectStore, prefix string) ([]FileItem, error) {
	var items []FileItem
	var token *string

	for {
		input := &s3.ListObjectsV2Input{
			Bucket: aws.String(store.Bucket),
			Prefix: aws.String(prefix),
		}
		input.Delimiter = aws.String("/")
		if token != nil {
			input.ContinuationToken = token
		}

		resp, err := client.ListObjectsV2(ctx, input)
		if err != nil {
			return nil, err
		}

		for _, obj := range resp.Contents {
			key := aws.ToString(obj.Key)
			rel := relativeS3Path(store.Prefix, key)
			if rel == "" {
				continue
			}
			if strings.HasSuffix(rel, "/") {
				continue
			}
			if strings.Contains(rel, "/") || strings.Contains(rel, "\\") {
				continue
			}

			item := FileItem{
				StoreType: storeTypeObject,
				FileName:  rel,
				Size:      aws.ToInt64(obj.Size),
			}
			if obj.LastModified != nil {
				item.LastModified = obj.LastModified.UTC().Format(timeFormat)
			}
			if obj.ETag != nil {
				item.ETag = strings.Trim(*obj.ETag, "\"")
			}
			items = append(items, item)
		}

		if !aws.ToBool(resp.IsTruncated) {
			break
		}
		token = resp.NextContinuationToken
	}

	return items, nil
}

func listBlockFiles(root string) ([]FileItem, error) {
	var items []FileItem

	info, err := os.Stat(root)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		items = append(items, FileItem{
			StoreType:    storeTypeBlock,
			FileName:     filepath.Base(root),
			Size:         info.Size(),
			LastModified: info.ModTime().UTC().Format(timeFormat),
		})
		return items, nil
	}

	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		entryInfo, err := entry.Info()
		if err != nil {
			return nil, err
		}
		items = append(items, FileItem{
			StoreType:    storeTypeBlock,
			FileName:     entry.Name(),
			Size:         entryInfo.Size(),
			LastModified: entryInfo.ModTime().UTC().Format(timeFormat),
		})
	}
	return items, nil
}
