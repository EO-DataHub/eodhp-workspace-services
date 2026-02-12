package services

import (
	"context"
	"fmt"
	"mime/multipart"
	"os"
	"path"
	"path/filepath"
	"strings"

	ws_manager "github.com/EO-DataHub/eodhp-workspace-manager/models"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

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

// Select the active object store (single-store assumption).
func selectObjectStore(stores []ws_manager.ObjectStore) (ws_manager.ObjectStore, error) {
	if len(stores) == 0 {
		return ws_manager.ObjectStore{}, fmt.Errorf("no object store configured")
	}
	return stores[0], nil
}

// Select the active block store (single-store assumption).
func selectBlockStore(stores []ws_manager.BlockStore) (ws_manager.BlockStore, error) {
	if len(stores) == 0 {
		return ws_manager.BlockStore{}, fmt.Errorf("no block store configured")
	}
	return stores[0], nil
}

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

func safeBlockPath(root, rel string) (string, error) {
	cleanRoot := filepath.Clean(root)
	if cleanRoot == "." || cleanRoot == "" {
		return "", fmt.Errorf("invalid block store root")
	}
	if rel == "" {
		return cleanRoot, nil
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
	cleanRel := filepath.Clean(rel)
	if cleanRel == "." || cleanRel == "" {
		return "", fmt.Errorf("invalid relative path")
	}
	if cleanRel == ".." || strings.HasPrefix(cleanRel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("invalid relative path")
	}
	full := filepath.Join(cleanRoot, cleanRel)
	relToRoot, err := filepath.Rel(cleanRoot, full)
	if err != nil {
		return "", err
	}
	if relToRoot == ".." || strings.HasPrefix(relToRoot, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path escapes workspace root")
	}
	return full, nil
}

func validateFileName(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("file name is required")
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

func listBlockFiles(root, timeFormat string) ([]FileItem, error) {
	var items []FileItem

	info, err := os.Stat(root)
	if err != nil {
		if os.IsNotExist(err) {
			return items, nil
		}
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("block store root is not a directory")
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

func (svc *FileService) responseTimeFormat() string {
	if svc != nil && svc.Config != nil {
		if format := strings.TrimSpace(svc.Config.Files.ResponseTimeFormat); format != "" {
			return format
		}
	}
	return defaultTimeFormat
}

func (svc *FileService) maxUploadFormMemoryBytes() int64 {
	if svc != nil && svc.Config != nil && svc.Config.Files.MaxUploadFormMemory > 0 {
		return svc.Config.Files.MaxUploadFormMemory << 20
	}
	return defaultFormMemory
}
