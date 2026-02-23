package services

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	ws_manager "github.com/EO-DataHub/eodhp-workspace-manager/models"
	"github.com/EO-DataHub/eodhp-workspace-services/internal/appconfig"
	"github.com/stretchr/testify/require"
)

func TestListBlockStoreItemsNoStoreConfigured(t *testing.T) {
	svc := FileService{}
	_, err := svc.listBlockStoreItems(context.Background(), nil, "ws-1")
	require.EqualError(t, err, "no block store configured")
}

func TestListBlockStoreItemsSuccess(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodGet, r.Method)
		require.Equal(t, "/ws-1/", r.URL.Path)

		require.NoError(t, json.NewEncoder(w).Encode([]map[string]any{
			{
				"name":  "my-file.tif",
				"type":  "file",
				"mtime": "Wed, 11 Feb 2026 12:53:04 GMT",
				"size":  12,
			},
		}))
	}))
	defer ts.Close()

	svc := FileService{
		Config: &appconfig.Config{
			Files: appconfig.FilesConfig{
				BlockBaseURL: ts.URL,
			},
		},
	}
	items, err := svc.listBlockStoreItems(context.Background(), []ws_manager.BlockStore{
		{MountPoint: "/ws-1"},
	}, "ws-1")
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, "my-file.tif", items[0].FileName)
	require.Equal(t, storeTypeBlock, items[0].StoreType)
}

func TestUploadBlockStoreFilesSuccess(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPut, r.Method)
		require.Equal(t, "/ws-1/upload.tif", r.URL.Path)
		w.WriteHeader(http.StatusCreated)
	}))
	defer ts.Close()

	svc := FileService{
		Config: &appconfig.Config{
			Files: appconfig.FilesConfig{
				BlockBaseURL: ts.URL,
			},
		},
	}
	files := mustBuildMultipartFiles(t, "files", "upload.tif", []byte("abc"))

	items, err := svc.uploadBlockStoreFiles(
		context.Background(),
		"ws-1",
		ws_manager.BlockStore{MountPoint: "/ws-1"},
		files,
	)
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, "upload.tif", items[0].FileName)
	require.Equal(t, int64(3), items[0].Size)
}

func TestDeleteBlockStoreFilesCollectsDeletedAndFailed(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodDelete, r.Method)
		switch r.URL.Path {
		case "/ws-1/good.tif":
			w.WriteHeader(http.StatusNoContent)
		case "/ws-1/missing.tif":
			w.WriteHeader(http.StatusNotFound)
		default:
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer ts.Close()

	svc := FileService{
		Config: &appconfig.Config{
			Files: appconfig.FilesConfig{
				BlockBaseURL: ts.URL,
			},
		},
	}
	deleted, failed, err := svc.deleteBlockStoreFiles(
		context.Background(),
		"ws-1",
		ws_manager.BlockStore{MountPoint: "/ws-1"},
		[]string{"bad/name.tif", "good.tif", "missing.tif"},
	)
	require.NoError(t, err)
	require.Equal(t, []string{"good.tif"}, deleted)
	require.Len(t, failed, 2)
	require.Equal(t, "bad/name.tif", failed[0].FileName)
	require.Equal(t, "missing.tif", failed[1].FileName)
}

func TestGetBlockStoreMetadataSuccess(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodHead, r.Method)
		require.Equal(t, "/ws-1/meta.tif", r.URL.Path)
		w.Header().Set("Content-Length", "20")
		w.Header().Set("Last-Modified", "Wed, 11 Feb 2026 12:53:04 GMT")
		w.Header().Set("ETag", `"etag-2"`)
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	svc := FileService{
		Config: &appconfig.Config{
			Files: appconfig.FilesConfig{
				BlockBaseURL: ts.URL,
			},
		},
	}
	item, err := svc.getBlockStoreMetadata(
		context.Background(),
		"ws-1",
		ws_manager.BlockStore{MountPoint: "/ws-1"},
		"meta.tif",
	)
	require.NoError(t, err)
	require.Equal(t, "meta.tif", item.FileName)
	require.Equal(t, int64(20), item.Size)
	require.Equal(t, "etag-2", item.ETag)
}

func TestBlockBaseURLAndTimeoutFromConfig(t *testing.T) {
	defaultSvc := &FileService{}
	svc := FileService{
		Config: &appconfig.Config{
			Files: appconfig.FilesConfig{
				BlockBaseURL:        " http://efs-nginx:80 ",
				BlockTimeoutSeconds: 12,
			},
		},
	}
	require.Equal(t, "http://efs-nginx:80", svc.blockBaseURL())
	require.Equal(t, defaultBlockTimeout, defaultSvc.blockTimeout())
	require.Equal(t, 12*time.Second, svc.blockTimeout())
}

func mustBuildMultipartFiles(t *testing.T, fieldName, fileName string, data []byte) []*multipart.FileHeader {
	t.Helper()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	part, err := writer.CreateFormFile(fieldName, fileName)
	require.NoError(t, err)
	_, err = part.Write(data)
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	req := httptest.NewRequest(http.MethodPost, "/upload", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	require.NoError(t, req.ParseMultipartForm(int64(len(data))+1024))

	return collectMultipartFiles(req.MultipartForm)
}
