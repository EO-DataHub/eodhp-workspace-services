package services

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestNewBlockNginxClientValidatesBaseURL(t *testing.T) {
	_, err := newBlockNginxClient("", 0, defaultTimeFormat)
	require.Error(t, err)

	_, err = newBlockNginxClient("://bad-url", 0, defaultTimeFormat)
	require.Error(t, err)

	_, err = newBlockNginxClient("efs-nginx", 0, defaultTimeFormat)
	require.Error(t, err)
}

func TestNewBlockNginxClientAppliesDefaults(t *testing.T) {
	client, err := newBlockNginxClient("http://efs-nginx:80/", 0, defaultTimeFormat)
	require.NoError(t, err)
	require.Equal(t, "http://efs-nginx:80", client.baseURL)
	require.Equal(t, defaultBlockTimeout, client.httpClient.Timeout)
	require.Equal(t, defaultTimeFormat, client.timeFormat)
}

func TestBlockNginxClientListFilesFiltersAndNormalizes(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodGet, r.Method)
		require.Equal(t, "/ws-1/", r.URL.Path)

		require.NoError(t, json.NewEncoder(w).Encode([]nginxAutoindexEntry{
			{
				Name:  "subdir",
				Type:  "directory",
				MTime: "Wed, 11 Feb 2026 12:53:04 GMT",
				Size:  json.RawMessage(`"-"`),
			},
			{
				Name:  "my-file.tif",
				Type:  "file",
				MTime: "Wed, 11 Feb 2026 12:53:04 GMT",
				Size:  json.RawMessage(`"123"`),
			},
			{
				Name:  "bad/name.tif",
				Type:  "file",
				MTime: "Wed, 11 Feb 2026 12:53:04 GMT",
				Size:  json.RawMessage(`123`),
			},
		}))
	}))
	defer ts.Close()

	client, err := newBlockNginxClient(ts.URL, 0, defaultTimeFormat)
	require.NoError(t, err)

	items, err := client.listFiles(context.Background(), "ws-1")
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, storeTypeBlock, items[0].StoreType)
	require.Equal(t, "my-file.tif", items[0].FileName)
	require.Equal(t, int64(123), items[0].Size)
	require.Equal(t, "2026-02-11T12:53:04Z", items[0].LastModified)
}

func TestBlockNginxClientListFilesNotFoundReturnsEmpty(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	client, err := newBlockNginxClient(ts.URL, 0, defaultTimeFormat)
	require.NoError(t, err)

	items, err := client.listFiles(context.Background(), "ws-1")
	require.NoError(t, err)
	require.Len(t, items, 0)
}

func TestBlockNginxClientListFilesUnexpectedStatusReturnsError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	client, err := newBlockNginxClient(ts.URL, 0, defaultTimeFormat)
	require.NoError(t, err)

	_, err = client.listFiles(context.Background(), "ws-1")
	require.Error(t, err)
	require.Contains(t, err.Error(), "block list failed with status 500")
}

func TestBlockNginxClientUploadFileUsesDefaultContentType(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPut, r.Method)
		require.Equal(t, "/ws-1/test.tif", r.URL.Path)
		require.Equal(t, "application/octet-stream", r.Header.Get("Content-Type"))
		w.WriteHeader(http.StatusCreated)
	}))
	defer ts.Close()

	client, err := newBlockNginxClient(ts.URL, 0, defaultTimeFormat)
	require.NoError(t, err)

	item, err := client.uploadFile(context.Background(), "ws-1", "test.tif", strings.NewReader("abc"), "")
	require.NoError(t, err)
	require.Equal(t, storeTypeBlock, item.StoreType)
	require.Equal(t, "test.tif", item.FileName)
}

func TestBlockNginxClientUploadFileFailureStatusReturnsError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	client, err := newBlockNginxClient(ts.URL, 0, defaultTimeFormat)
	require.NoError(t, err)

	_, err = client.uploadFile(context.Background(), "ws-1", "test.tif", strings.NewReader("abc"), "image/tiff")
	require.Error(t, err)
	require.Contains(t, err.Error(), "block upload failed with status 500")
}

func TestBlockNginxClientDeleteFileNotFoundReturnsError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodDelete, r.Method)
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	client, err := newBlockNginxClient(ts.URL, 0, defaultTimeFormat)
	require.NoError(t, err)

	err = client.deleteFile(context.Background(), "ws-1", "test.tif")
	require.Error(t, err)
	require.Equal(t, "file not found", err.Error())
}

func TestBlockNginxClientFileMetadataParsesHeaders(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodHead, r.Method)
		require.Equal(t, "/ws-1/test.tif", r.URL.Path)
		w.Header().Set("Content-Length", "42")
		w.Header().Set("Last-Modified", "Wed, 11 Feb 2026 12:53:04 GMT")
		w.Header().Set("ETag", `"etag-1"`)
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	client, err := newBlockNginxClient(ts.URL, 0, defaultTimeFormat)
	require.NoError(t, err)

	item, err := client.fileMetadata(context.Background(), "ws-1", "test.tif")
	require.NoError(t, err)
	require.Equal(t, storeTypeBlock, item.StoreType)
	require.Equal(t, "test.tif", item.FileName)
	require.Equal(t, int64(42), item.Size)
	require.Equal(t, "2026-02-11T12:53:04Z", item.LastModified)
	require.Equal(t, "etag-1", item.ETag)
}

func TestBlockNginxClientFileMetadataFallsBackToDateHeader(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Last-Modified", "invalid")
		w.Header().Set("Date", "Wed, 11 Feb 2026 12:53:04 GMT")
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	client, err := newBlockNginxClient(ts.URL, 0, defaultTimeFormat)
	require.NoError(t, err)

	item, err := client.fileMetadata(context.Background(), "ws-1", "test.tif")
	require.NoError(t, err)
	require.Equal(t, int64(0), item.Size)
	require.Equal(t, "2026-02-11T12:53:04Z", item.LastModified)
}

func TestBlockNginxClientWorkspaceURLValidationAndEscaping(t *testing.T) {
	client, err := newBlockNginxClient("http://efs-nginx:80/base", 0, defaultTimeFormat)
	require.NoError(t, err)

	_, err = client.workspaceURL("", "", true)
	require.Error(t, err)

	_, err = client.workspaceURL("bad/ws", "", true)
	require.Error(t, err)

	u, err := client.workspaceURL("ws-1", "my-file.tif", false)
	require.NoError(t, err)
	require.Equal(t, "http://efs-nginx:80/base/ws-1/my-file.tif", u)

	u, err = client.workspaceURL("ws-1", "", true)
	require.NoError(t, err)
	require.Equal(t, "http://efs-nginx:80/base/ws-1/", u)
}

func TestParseAutoindexSize(t *testing.T) {
	tests := []struct {
		name string
		raw  json.RawMessage
		want int64
	}{
		{name: "empty", raw: nil, want: 0},
		{name: "number", raw: json.RawMessage(`123.9`), want: 123},
		{name: "string number", raw: json.RawMessage(`"456"`), want: 456},
		{name: "dash", raw: json.RawMessage(`"-"`), want: 0},
		{name: "negative", raw: json.RawMessage(`"-8"`), want: 0},
		{name: "invalid", raw: json.RawMessage(`"abc"`), want: 0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, parseAutoindexSize(tc.raw))
		})
	}
}

func TestTimeParsingHelpers(t *testing.T) {
	formatted := formatAutoindexTime("Wed, 11 Feb 2026 12:53:04 GMT", defaultTimeFormat)
	require.Equal(t, "2026-02-11T12:53:04Z", formatted)

	fallback := formatAutoindexTime("not-a-date", defaultTimeFormat)
	require.Equal(t, "not-a-date", fallback)

	require.Equal(t, "", formatHeaderTime("bad-date", defaultTimeFormat))
	require.Equal(t, "2026-02-11T12:53:04Z", formatHeaderTime("Wed, 11 Feb 2026 12:53:04 GMT", defaultTimeFormat))

	parsed, err := parseNginxTime("2026-02-11T12:53:04Z")
	require.NoError(t, err)
	require.Equal(t, "2026-02-11T12:53:04Z", parsed.UTC().Format(time.RFC3339))

	_, err = parseNginxTime("definitely-invalid")
	require.Error(t, err)
	require.Equal(t, fmt.Errorf("unsupported time format").Error(), err.Error())
}
