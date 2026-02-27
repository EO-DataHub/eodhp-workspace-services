package services

import (
	"context"
	"fmt"
	"mime/multipart"
	"path"
	"strings"

	ws_manager "github.com/EO-DataHub/eodhp-workspace-manager/models"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

const maxFileNameBytes = 255

// collectStores flattens object and block stores from workspace settings.
func collectStores(workspace *ws_manager.WorkspaceSettings) ([]ws_manager.ObjectStore, []ws_manager.BlockStore) {
	var objectStores []ws_manager.ObjectStore
	var blockStores []ws_manager.BlockStore
	if workspace == nil || workspace.Stores == nil {
		return objectStores, blockStores
	}
	for _, store := range *workspace.Stores {
		objectStores = append(objectStores, store.Object...)
		blockStores = append(blockStores, store.Block...)
	}
	return objectStores, blockStores
}

// selectObjectStore returns the configured object store when exactly one is present.
func selectObjectStore(stores []ws_manager.ObjectStore) (ws_manager.ObjectStore, error) {
	if len(stores) == 0 {
		return ws_manager.ObjectStore{}, fmt.Errorf("no object store configured")
	}
	if len(stores) > 1 {
		return ws_manager.ObjectStore{}, fmt.Errorf("multiple object stores configured; expected exactly one")
	}
	return stores[0], nil
}

// selectBlockStore returns the configured block store when exactly one is present.
func selectBlockStore(stores []ws_manager.BlockStore) (ws_manager.BlockStore, error) {
	if len(stores) == 0 {
		return ws_manager.BlockStore{}, fmt.Errorf("no block store configured")
	}
	if len(stores) > 1 {
		return ws_manager.BlockStore{}, fmt.Errorf("multiple block stores configured; expected exactly one")
	}
	return stores[0], nil
}

// resolveBlockWorkspaceDir derives and validates the block store workspace directory name.
func resolveBlockWorkspaceDir(store ws_manager.BlockStore, workspaceID string) (string, error) {
	workspaceID = strings.TrimSpace(workspaceID)
	if workspaceID == "" {
		return "", fmt.Errorf("workspace id is required")
	}

	mountPoint := strings.TrimSpace(store.MountPoint)
	if mountPoint == "" {
		return "", fmt.Errorf("block store not provisioned")
	}

	cleanMount := path.Clean(strings.ReplaceAll(mountPoint, "\\", "/"))
	workspaceDir := path.Base(cleanMount)
	if workspaceDir == "." || workspaceDir == "/" || workspaceDir == "" {
		return "", fmt.Errorf("invalid block store mount point")
	}
	if err := validateFileName(workspaceDir); err != nil {
		return "", fmt.Errorf("invalid block store mount point")
	}

	if workspaceDir != workspaceID {
		return "", fmt.Errorf("block store mount point does not match workspace")
	}

	return workspaceDir, nil
}

// collectMultipartFiles extracts non-nil file headers from a parsed multipart form.
func collectMultipartFiles(form *multipart.Form) []*multipart.FileHeader {
	if form == nil || form.File == nil {
		return nil
	}
	var out []*multipart.FileHeader
	for _, headers := range form.File {
		for _, fh := range headers {
			if fh != nil {
				out = append(out, fh)
			}
		}
	}
	return out
}

// safeS3Key validates a file name and returns its normalized key under the given prefix.
func safeS3Key(prefix, rel string) (string, error) {
	rel = strings.TrimSpace(rel)
	if rel == "" {
		return "", fmt.Errorf("file name is required")
	}
	if strings.Contains(rel, "\\") {
		return "", fmt.Errorf("invalid path separator")
	}
	if strings.Contains(rel, "/") {
		return "", fmt.Errorf("nested paths are not supported")
	}
	if strings.HasPrefix(rel, ".") {
		return "", fmt.Errorf("invalid file name")
	}
	cleaned := path.Clean("/" + rel)
	if cleaned == "/" || strings.Contains(cleaned, "..") {
		return "", fmt.Errorf("invalid relative path")
	}
	cleaned = strings.TrimPrefix(cleaned, "/")
	if cleaned == "" {
		return "", fmt.Errorf("invalid relative path")
	}

	base := strings.Trim(prefix, "/")
	if base == "" {
		return cleaned, nil
	}
	return path.Join(base, cleaned), nil
}

// safeS3Prefix validates and normalizes an S3 prefix for list operations.
func safeS3Prefix(prefix, rel string) (string, error) {
	base := strings.Trim(prefix, "/")
	if base == "" {
		return "", fmt.Errorf("object prefix is required")
	}
	if rel == "" {
		return base + "/", nil
	}
	if strings.Contains(rel, "\\") {
		return "", fmt.Errorf("invalid path separator")
	}
	cleaned := path.Clean("/" + rel)
	if strings.Contains(cleaned, "..") {
		return "", fmt.Errorf("invalid relative path")
	}
	cleaned = strings.Trim(cleaned, "/")
	if cleaned == "." {
		cleaned = ""
	}
	if cleaned == "" {
		return base + "/", nil
	}
	return path.Join(base, cleaned) + "/", nil
}

// relativeS3Path returns the key relative to the configured store prefix.
func relativeS3Path(prefix, key string) string {
	base := strings.Trim(prefix, "/")
	if base == "" {
		return strings.Trim(key, "/")
	}
	key = strings.Trim(key, "/")
	basePrefix := base + "/"
	if strings.HasPrefix(key, basePrefix) {
		return strings.TrimPrefix(key, basePrefix)
	}
	return key
}

// validateFileName validates a single-level file name for upload, delete, and metadata operations.
func validateFileName(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("file name is required")
	}
	// Keep a conservative cross-store limit. Block/file-system backends commonly cap a single
	// path segment at 255 bytes, while object stores may allow longer keys.
	if len(name) > maxFileNameBytes {
		return fmt.Errorf("file name too long")
	}
	if strings.Contains(name, "\\") {
		return fmt.Errorf("invalid path separator")
	}
	if strings.Contains(name, "/") {
		return fmt.Errorf("nested paths are not supported")
	}
	if strings.HasPrefix(name, ".") {
		return fmt.Errorf("invalid file name")
	}
	if strings.Contains(name, "..") {
		return fmt.Errorf("invalid file name")
	}
	return nil
}

// listS3Objects lists objects for a prefix and maps them into file items.
func listS3Objects(ctx context.Context, client *s3.Client, store ws_manager.ObjectStore, prefix, timeFormat string) ([]FileItem, error) {
	var items []FileItem
	var token *string

	for {
		out, err := client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
			Bucket:            aws.String(store.Bucket),
			Prefix:            aws.String(prefix),
			ContinuationToken: token,
		})
		if err != nil {
			return nil, err
		}
		for _, obj := range out.Contents {
			key := aws.ToString(obj.Key)
			if key == "" || strings.HasSuffix(key, "/") {
				continue
			}
			relative := relativeS3Path(store.Prefix, key)
			if strings.TrimSpace(relative) == "" {
				continue
			}
			if strings.Contains(relative, "/") {
				continue
			}
			item := FileItem{
				StoreType: storeTypeObject,
				FileName:  relative,
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
		if out.IsTruncated == nil || !*out.IsTruncated {
			break
		}
		token = out.NextContinuationToken
	}

	return items, nil
}

// extractBearerToken extracts a bearer token from an Authorization header.
func extractBearerToken(authHeader string) string {
	authHeader = strings.TrimSpace(authHeader)
	if authHeader == "" {
		return ""
	}
	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return ""
	}
	token := strings.TrimSpace(parts[1])
	if token == "" {
		return ""
	}
	return token
}

// responseTimeFormat returns the configured API response time format or a default value.
func (svc *FileService) responseTimeFormat() string {
	if svc != nil && svc.Config != nil {
		if format := strings.TrimSpace(svc.Config.Files.ResponseTimeFormat); format != "" {
			return format
		}
	}
	return defaultTimeFormat
}

// maxUploadFormMemoryBytes returns the configured multipart form memory limit in bytes.
func (svc *FileService) maxUploadFormMemoryBytes() int64 {
	if svc != nil && svc.Config != nil && svc.Config.Files.MaxUploadFormMemory > 0 {
		return svc.Config.Files.MaxUploadFormMemory << 20
	}
	return defaultFormMemory
}
