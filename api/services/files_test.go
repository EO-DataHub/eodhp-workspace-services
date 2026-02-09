package services

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"
	"time"

	"github.com/EO-DataHub/eodhp-workspace-services/internal/appconfig"
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

func TestBlockDownloadSignature(t *testing.T) {
	svc := FileService{
		Config: &appconfig.Config{
			Download: appconfig.DownloadConfig{
				SigningSecret: "test-secret",
			},
		},
	}

	exp := time.Now().Add(5 * time.Minute).Unix()
	sig := signBlockDownload("test-secret", "ws-1", "file.tif", exp)

	err := svc.validateBlockDownloadSignature("ws-1", "file.tif", strconv.FormatInt(exp, 10), sig)
	require.NoError(t, err)

	expired := time.Now().Add(-1 * time.Minute).Unix()
	err = svc.validateBlockDownloadSignature("ws-1", "file.tif", strconv.FormatInt(expired, 10), sig)
	require.Error(t, err)

	err = svc.validateBlockDownloadSignature("ws-1", "file.tif", strconv.FormatInt(exp, 10), "bad")
	require.Error(t, err)
}

func TestBlockDownloadURL(t *testing.T) {
	svc := FileService{
		Config: &appconfig.Config{
			Host:     "example.com",
			BasePath: "/api",
			Download: appconfig.DownloadConfig{
				SigningSecret: "test-secret",
			},
		},
	}

	req := httptest.NewRequest(http.MethodGet, "http://example.com", nil)
	urlStr := svc.blockDownloadURL(req, "ws-1", "file.tif")
	require.NotEmpty(t, urlStr)

	parsed, err := url.Parse(urlStr)
	require.NoError(t, err)
	require.Equal(t, "http", parsed.Scheme)
	require.Equal(t, "example.com", parsed.Host)
	require.Equal(t, "/api/workspaces/ws-1/files/block/download", parsed.Path)

	q := parsed.Query()
	require.Equal(t, "file.tif", q.Get("file"))
	require.NotEmpty(t, q.Get("exp"))
	require.NotEmpty(t, q.Get("sig"))

	exp, err := strconv.ParseInt(q.Get("exp"), 10, 64)
	require.NoError(t, err)
	expectedSig := signBlockDownload("test-secret", "ws-1", "file.tif", exp)
	require.Equal(t, expectedSig, q.Get("sig"))

	svc.Config.Download.SigningSecret = ""
	require.Empty(t, svc.blockDownloadURL(req, "ws-1", "file.tif"))
}
