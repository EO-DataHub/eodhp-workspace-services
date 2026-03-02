package services

import (
	"mime/multipart"
	"strings"
	"testing"

	ws_manager "github.com/EO-DataHub/eodhp-workspace-manager/models"
	"github.com/stretchr/testify/require"
)

func TestCollectStores(t *testing.T) {
	t.Run("nil workspace", func(t *testing.T) {
		obj, blk := collectStores(nil)
		require.Empty(t, obj)
		require.Empty(t, blk)
	})

	t.Run("workspace without stores", func(t *testing.T) {
		workspace := &ws_manager.WorkspaceSettings{}
		obj, blk := collectStores(workspace)
		require.Empty(t, obj)
		require.Empty(t, blk)
	})

	t.Run("collect object and block stores", func(t *testing.T) {
		stores := []ws_manager.Stores{
			{
				Object: []ws_manager.ObjectStore{
					{Bucket: "bucket-a", Prefix: "prefix-a"},
				},
				Block: []ws_manager.BlockStore{
					{MountPoint: "/mnt/ws-1"},
				},
			},
			{
				Object: []ws_manager.ObjectStore{
					{Bucket: "bucket-b", Prefix: "prefix-b"},
				},
				Block: []ws_manager.BlockStore{
					{MountPoint: "/mnt/ws-2"},
				},
			},
		}
		workspace := &ws_manager.WorkspaceSettings{Stores: &stores}

		obj, blk := collectStores(workspace)
		require.Len(t, obj, 2)
		require.Len(t, blk, 2)
		require.Equal(t, "bucket-a", obj[0].Bucket)
		require.Equal(t, "bucket-b", obj[1].Bucket)
		require.Equal(t, "/mnt/ws-1", blk[0].MountPoint)
		require.Equal(t, "/mnt/ws-2", blk[1].MountPoint)
	})
}

func TestSelectObjectStore(t *testing.T) {
	_, err := selectObjectStore(nil)
	require.EqualError(t, err, "no object store configured")

	store, err := selectObjectStore([]ws_manager.ObjectStore{{Bucket: "bucket-a", Prefix: "prefix-a"}})
	require.NoError(t, err)
	require.Equal(t, "bucket-a", store.Bucket)

	_, err = selectObjectStore([]ws_manager.ObjectStore{
		{Bucket: "bucket-a", Prefix: "prefix-a"},
		{Bucket: "bucket-b", Prefix: "prefix-b"},
	})
	require.EqualError(t, err, "multiple object stores configured; expected exactly one")
}

func TestSelectBlockStore(t *testing.T) {
	_, err := selectBlockStore(nil)
	require.EqualError(t, err, "no block store configured")

	store, err := selectBlockStore([]ws_manager.BlockStore{{MountPoint: "/mnt/ws-1"}})
	require.NoError(t, err)
	require.Equal(t, "/mnt/ws-1", store.MountPoint)

	_, err = selectBlockStore([]ws_manager.BlockStore{
		{MountPoint: "/mnt/ws-1"},
		{MountPoint: "/mnt/ws-2"},
	})
	require.EqualError(t, err, "multiple block stores configured; expected exactly one")
}

func TestResolveBlockWorkspaceDir(t *testing.T) {
	_, err := resolveBlockWorkspaceDir(ws_manager.BlockStore{MountPoint: "/mnt/ws-1"}, "")
	require.EqualError(t, err, "workspace id is required")

	_, err = resolveBlockWorkspaceDir(ws_manager.BlockStore{}, "ws-1")
	require.EqualError(t, err, "block store not provisioned")

	_, err = resolveBlockWorkspaceDir(ws_manager.BlockStore{MountPoint: "/"}, "ws-1")
	require.EqualError(t, err, "invalid block store mount point")

	_, err = resolveBlockWorkspaceDir(ws_manager.BlockStore{MountPoint: "/mnt/ws-1"}, "ws-2")
	require.EqualError(t, err, "block store mount point does not match workspace")

	dir, err := resolveBlockWorkspaceDir(ws_manager.BlockStore{MountPoint: "/mnt/ws-1"}, "ws-1")
	require.NoError(t, err)
	require.Equal(t, "ws-1", dir)
}

func TestSafeS3Key(t *testing.T) {
	key, err := safeS3Key("workspace/ws-1", "file.tif")
	require.NoError(t, err)
	require.Equal(t, "workspace/ws-1/file.tif", key)

	key, err = safeS3Key("", "file.tif")
	require.NoError(t, err)
	require.Equal(t, "file.tif", key)

	_, err = safeS3Key("workspace/ws-1", "")
	require.EqualError(t, err, "file name is required")

	_, err = safeS3Key("workspace/ws-1", "bad\\name.tif")
	require.EqualError(t, err, "invalid path separator")

	_, err = safeS3Key("workspace/ws-1", "bad/name.tif")
	require.EqualError(t, err, "nested paths are not supported")

	_, err = safeS3Key("workspace/ws-1", ".hidden")
	require.EqualError(t, err, "invalid file name")
}

func TestSafeS3Prefix(t *testing.T) {
	prefix, err := safeS3Prefix("workspace/ws-1", "")
	require.NoError(t, err)
	require.Equal(t, "workspace/ws-1/", prefix)

	prefix, err = safeS3Prefix("/workspace/ws-1/", "subdir")
	require.NoError(t, err)
	require.Equal(t, "workspace/ws-1/subdir/", prefix)

	_, err = safeS3Prefix("", "")
	require.EqualError(t, err, "object prefix is required")

	_, err = safeS3Prefix("workspace/ws-1", "bad\\dir")
	require.EqualError(t, err, "invalid path separator")
}

func TestRelativeS3Path(t *testing.T) {
	require.Equal(t, "file.tif", relativeS3Path("workspace/ws-1", "workspace/ws-1/file.tif"))
	require.Equal(t, "other/file.tif", relativeS3Path("workspace/ws-1", "/other/file.tif/"))
	require.Equal(t, "file.tif", relativeS3Path("", "/file.tif/"))
}

func TestExtractBearerToken(t *testing.T) {
	require.Equal(t, "", extractBearerToken(""))
	require.Equal(t, "", extractBearerToken("Basic abc"))
	require.Equal(t, "", extractBearerToken("Bearer "))
	require.Equal(t, "token-1", extractBearerToken("Bearer token-1"))
	require.Equal(t, "token-2", extractBearerToken("bearer token-2"))
}

func TestCollectMultipartFiles(t *testing.T) {
	require.Nil(t, collectMultipartFiles(nil))
	require.Nil(t, collectMultipartFiles(&multipart.Form{}))

	fh1 := &multipart.FileHeader{Filename: "a.tif"}
	fh2 := &multipart.FileHeader{Filename: "b.tif"}
	form := &multipart.Form{
		File: map[string][]*multipart.FileHeader{
			"files": {fh1, nil},
			"more":  {fh2},
		},
	}

	files := collectMultipartFiles(form)
	require.Len(t, files, 2)
	got := []string{files[0].Filename, files[1].Filename}
	require.ElementsMatch(t, []string{"a.tif", "b.tif"}, got)
}

func TestValidateFileNameLengthLimit(t *testing.T) {
	valid := strings.Repeat("a", maxFileNameBytes)
	require.NoError(t, validateFileName(valid))

	tooLong := strings.Repeat("a", maxFileNameBytes+1)
	require.EqualError(t, validateFileName(tooLong), "file name too long")
}
