package services

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"
)

const defaultBlockTimeout = 30 * time.Second

type blockNginxClient struct {
	baseURL    string
	httpClient *http.Client
	timeFormat string
}

type nginxAutoindexEntry struct {
	Name  string          `json:"name"`
	Type  string          `json:"type"`
	MTime string          `json:"mtime"`
	Size  json.RawMessage `json:"size"`
}

// newBlockNginxClient creates a block store HTTP client from base URL and timeout settings.
func newBlockNginxClient(baseURL string, timeout time.Duration, timeFormat string) (*blockNginxClient, error) {
	baseURL = strings.TrimSpace(baseURL)
	if baseURL == "" {
		return nil, fmt.Errorf("files.blockBaseUrl is required")
	}

	parsed, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid files.blockBaseUrl: %w", err)
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return nil, fmt.Errorf("invalid files.blockBaseUrl")
	}

	if timeout <= 0 {
		timeout = defaultBlockTimeout
	}

	return &blockNginxClient{
		baseURL:    strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{Timeout: timeout},
		timeFormat: timeFormat,
	}, nil
}

// listFiles lists files under a workspace directory exposed by the block store proxy.
func (c *blockNginxClient) listFiles(ctx context.Context, workspaceID string) ([]FileItem, error) {
	listURL, err := c.workspaceURL(workspaceID, "", true)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, listURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return []FileItem{}, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("block list failed with status %d", resp.StatusCode)
	}

	var entries []nginxAutoindexEntry
	if err := json.NewDecoder(resp.Body).Decode(&entries); err != nil {
		return nil, fmt.Errorf("failed to decode block list response: %w", err)
	}

	items := make([]FileItem, 0, len(entries))
	for _, entry := range entries {
		if strings.EqualFold(entry.Type, "directory") {
			continue
		}
		if err := validateFileName(entry.Name); err != nil {
			continue
		}
		items = append(items, FileItem{
			StoreType:    storeTypeBlock,
			FileName:     entry.Name,
			Size:         parseAutoindexSize(entry.Size),
			LastModified: formatAutoindexTime(entry.MTime, c.timeFormat),
		})
	}

	return items, nil
}

// uploadFile uploads a single file to the block store proxy workspace path.
func (c *blockNginxClient) uploadFile(
	ctx context.Context,
	workspaceID string,
	fileName string,
	body io.Reader,
	contentType string,
) (FileItem, error) {
	if err := validateFileName(fileName); err != nil {
		return FileItem{}, err
	}

	fileURL, err := c.workspaceURL(workspaceID, fileName, false)
	if err != nil {
		return FileItem{}, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, fileURL, body)
	if err != nil {
		return FileItem{}, err
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	req.Header.Set("Content-Type", contentType)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return FileItem{}, err
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK, http.StatusCreated, http.StatusNoContent:
		return FileItem{
			StoreType: storeTypeBlock,
			FileName:  fileName,
		}, nil
	default:
		return FileItem{}, fmt.Errorf("block upload failed with status %d", resp.StatusCode)
	}
}

// deleteFile deletes a single file from the block store proxy workspace path.
func (c *blockNginxClient) deleteFile(ctx context.Context, workspaceID string, fileName string) error {
	if err := validateFileName(fileName); err != nil {
		return err
	}

	fileURL, err := c.workspaceURL(workspaceID, fileName, false)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, fileURL, nil)
	if err != nil {
		return err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK, http.StatusNoContent:
		return nil
	case http.StatusNotFound:
		return fmt.Errorf("file not found")
	default:
		return fmt.Errorf("block delete failed with status %d", resp.StatusCode)
	}
}

// fileMetadata reads metadata for a single file from block store proxy response headers.
func (c *blockNginxClient) fileMetadata(ctx context.Context, workspaceID string, fileName string) (FileItem, error) {
	if err := validateFileName(fileName); err != nil {
		return FileItem{}, err
	}

	fileURL, err := c.workspaceURL(workspaceID, fileName, false)
	if err != nil {
		return FileItem{}, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodHead, fileURL, nil)
	if err != nil {
		return FileItem{}, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return FileItem{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return FileItem{}, fmt.Errorf("file not found")
	}
	if resp.StatusCode != http.StatusOK {
		return FileItem{}, fmt.Errorf("block metadata failed with status %d", resp.StatusCode)
	}

	size := int64(0)
	if contentLength := strings.TrimSpace(resp.Header.Get("Content-Length")); contentLength != "" {
		parsedSize, err := strconv.ParseInt(contentLength, 10, 64)
		if err == nil && parsedSize >= 0 {
			size = parsedSize
		}
	}

	lastModified := formatHeaderTime(resp.Header.Get("Last-Modified"), c.timeFormat)
	if lastModified == "" {
		lastModified = formatHeaderTime(resp.Header.Get("Date"), c.timeFormat)
	}

	return FileItem{
		StoreType:    storeTypeBlock,
		FileName:     fileName,
		Size:         size,
		LastModified: lastModified,
		ETag:         strings.Trim(resp.Header.Get("ETag"), `"`),
	}, nil
}

// workspaceURL builds a block store URL for a workspace directory or file path.
func (c *blockNginxClient) workspaceURL(workspaceID string, fileName string, directory bool) (string, error) {
	workspaceID = strings.TrimSpace(workspaceID)
	if workspaceID == "" {
		return "", fmt.Errorf("workspace id is required")
	}
	if strings.Contains(workspaceID, "/") || strings.Contains(workspaceID, "\\") {
		return "", fmt.Errorf("invalid workspace id")
	}

	parsed, err := url.Parse(c.baseURL)
	if err != nil {
		return "", err
	}

	workspaceSegment := url.PathEscape(workspaceID)
	joinedPath := path.Join(parsed.Path, workspaceSegment)
	if fileName != "" {
		joinedPath = path.Join(joinedPath, url.PathEscape(fileName))
	}
	if directory {
		joinedPath = strings.TrimRight(joinedPath, "/") + "/"
	}

	parsed.Path = joinedPath
	return parsed.String(), nil
}

// parseAutoindexSize parses nginx autoindex size values from numeric or string JSON values.
func parseAutoindexSize(raw json.RawMessage) int64 {
	if len(raw) == 0 {
		return 0
	}

	var numberValue float64
	if err := json.Unmarshal(raw, &numberValue); err == nil {
		if numberValue > 0 {
			return int64(numberValue)
		}
		return 0
	}

	var stringValue string
	if err := json.Unmarshal(raw, &stringValue); err != nil {
		return 0
	}
	stringValue = strings.TrimSpace(stringValue)
	if stringValue == "" || stringValue == "-" {
		return 0
	}
	parsed, err := strconv.ParseInt(stringValue, 10, 64)
	if err != nil || parsed < 0 {
		return 0
	}
	return parsed
}

// formatAutoindexTime normalizes nginx autoindex time values to API response format.
func formatAutoindexTime(value string, timeFormat string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}

	parsed, err := parseNginxTime(value)
	if err != nil {
		return value
	}
	return parsed.UTC().Format(timeFormat)
}

// formatHeaderTime parses an RFC1123 header time and normalizes it to API response format.
func formatHeaderTime(value string, timeFormat string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	t, err := time.Parse(time.RFC1123, value)
	if err != nil {
		return ""
	}
	return t.UTC().Format(timeFormat)
}

// parseNginxTime parses known nginx timestamp layouts.
func parseNginxTime(value string) (time.Time, error) {
	layouts := []string{
		// Nginx autoindex commonly returns RFC1123-style timestamps (e.g. "Wed, 11 Feb 2026 12:53:04 GMT").
		// We parse these first so block lastModified can be normalized to the API response time format.
		time.RFC1123,
		time.RFC1123Z,
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05Z07:00",
		"2006-01-02 15:04:05",
	}

	for _, layout := range layouts {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed, nil
		}
	}
	return time.Time{}, fmt.Errorf("unsupported time format")
}
