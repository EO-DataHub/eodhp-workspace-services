package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/EO-DataHub/eodhp-workspace-services/api/middleware"
	"github.com/EO-DataHub/eodhp-workspace-services/aws"
	"github.com/EO-DataHub/eodhp-workspace-services/internal/authn"
	"github.com/stretchr/testify/assert"
)

type MockSTSClient struct{}

func (m *MockSTSClient) AssumeRoleWithWebIdentity(workspaceName string) (*aws.S3STSCredentialsResponse, error) {
	return &aws.S3STSCredentialsResponse{
		AccessKeyId:     "mockAccessKeyId",
		SecretAccessKey: "mockSecretAccessKey",
		SessionToken:    "mockSessionToken",
		Expiration:      "2024-11-13T16:36:20Z",
	}, nil
}

func TestGetS3Credentials(t *testing.T) {
	mockSTSClient := &MockSTSClient{}

	req, err := http.NewRequest("GET", "/api/workspaces/s3/credentials", nil)
	assert.NoError(t, err)

	claims := authn.Claims{Username: "test-user"}
	ctx := context.WithValue(req.Context(), middleware.ClaimsKey, claims)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler := GetS3Credentials(mockSTSClient)
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code, "handler returned wrong status code")
	assert.Equal(t, "application/json", rr.Header().Get("Content-Type"), "handler returned wrong content type")

	var creds aws.S3STSCredentialsResponse
	err = json.Unmarshal(rr.Body.Bytes(), &creds)

	assert.NoError(t, err, "failed to unmarshal response body")
	assert.Equal(t, "mockAccessKeyId", creds.AccessKeyId)
	assert.Equal(t, "mockSecretAccessKey", creds.SecretAccessKey)
	assert.Equal(t, "mockSessionToken", creds.SessionToken)
	assert.Equal(t, "2024-11-13T16:36:20Z", creds.Expiration)
}
