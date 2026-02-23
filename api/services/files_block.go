package services

import (
	"context"
	"fmt"
	"mime/multipart"
	"strings"
	"time"

	ws_manager "github.com/EO-DataHub/eodhp-workspace-manager/models"
)

// listBlockStoreItems lists files from the selected block store for a workspace.
func (svc *FileService) listBlockStoreItems(ctx context.Context, stores []ws_manager.BlockStore, workspaceID string) ([]FileItem, error) {
	if len(stores) == 0 {
		return nil, fmt.Errorf("no block store configured")
	}
	store, err := selectBlockStore(stores)
	if err != nil {
		return nil, err
	}
	workspaceDir, err := resolveBlockWorkspaceDir(store, workspaceID)
	if err != nil {
		return nil, err
	}
	client, err := svc.newBlockNginxClient()
	if err != nil {
		return nil, err
	}
	return client.listFiles(ctx, workspaceDir)
}

// uploadBlockStoreFiles uploads multipart files to the workspace block store.
func (svc *FileService) uploadBlockStoreFiles(
	ctx context.Context,
	workspaceID string,
	store ws_manager.BlockStore,
	files []*multipart.FileHeader,
) ([]FileItem, error) {
	workspaceDir, err := resolveBlockWorkspaceDir(store, workspaceID)
	if err != nil {
		return nil, err
	}
	client, err := svc.newBlockNginxClient()
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
		item, err := client.uploadFile(ctx, workspaceDir, fh.Filename, src, fh.Header.Get("Content-Type"))
		if closeErr := src.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
		if err != nil {
			return nil, err
		}
		if item.Size == 0 && fh.Size > 0 {
			item.Size = fh.Size
		}
		items = append(items, item)
	}

	return items, nil
}

// deleteBlockStoreFiles deletes block store files and reports per-file failures.
func (svc *FileService) deleteBlockStoreFiles(
	ctx context.Context,
	workspaceID string,
	store ws_manager.BlockStore,
	paths []string,
) ([]string, []FileFail, error) {
	workspaceDir, err := resolveBlockWorkspaceDir(store, workspaceID)
	if err != nil {
		return nil, nil, err
	}
	client, err := svc.newBlockNginxClient()
	if err != nil {
		return nil, nil, err
	}

	var deleted []string
	var failed []FileFail
	for _, p := range paths {
		if err := validateFileName(p); err != nil {
			failed = append(failed, FileFail{FileName: p, Error: err.Error()})
			continue
		}
		err := client.deleteFile(ctx, workspaceDir, p)
		if err != nil {
			failed = append(failed, FileFail{FileName: p, Error: err.Error()})
			continue
		}
		deleted = append(deleted, p)
	}

	return deleted, failed, nil
}

// getBlockStoreMetadata returns metadata for a file in the block store.
func (svc *FileService) getBlockStoreMetadata(
	ctx context.Context,
	workspaceID string,
	store ws_manager.BlockStore,
	pathParam string,
) (FileItem, error) {
	workspaceDir, err := resolveBlockWorkspaceDir(store, workspaceID)
	if err != nil {
		return FileItem{}, err
	}
	client, err := svc.newBlockNginxClient()
	if err != nil {
		return FileItem{}, err
	}
	return client.fileMetadata(ctx, workspaceDir, pathParam)
}

// newBlockNginxClient creates a block store HTTP client using file service configuration.
func (svc *FileService) newBlockNginxClient() (*blockNginxClient, error) {
	return newBlockNginxClient(
		svc.blockBaseURL(),
		svc.blockTimeout(),
		svc.responseTimeFormat(),
	)
}

// blockBaseURL returns the configured base URL for the block store proxy.
func (svc *FileService) blockBaseURL() string {
	if svc != nil && svc.Config != nil {
		return strings.TrimSpace(svc.Config.Files.BlockBaseURL)
	}
	return ""
}

// blockTimeout returns the configured block store timeout or the default timeout.
func (svc *FileService) blockTimeout() time.Duration {
	if svc != nil && svc.Config != nil && svc.Config.Files.BlockTimeoutSeconds > 0 {
		return time.Duration(svc.Config.Files.BlockTimeoutSeconds) * time.Second
	}
	return defaultBlockTimeout
}
