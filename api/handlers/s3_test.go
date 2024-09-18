package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/EO-DataHub/eodhp-workspace-services/aws"
)

func TestGetS3Credentials(t *testing.T) {
	req, err := http.NewRequest("GET", "/api/workspaces/s3/credentials", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(GetS3Credentials())

	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	expectedContentType := "application/json"
	if contentType := rr.Header().Get("Content-Type"); contentType != expectedContentType {
		t.Errorf("handler returned wrong content type: got %v want %v", contentType, expectedContentType)
	}

	var creds aws.S3STSCredentialsResponse
	err = json.Unmarshal(rr.Body.Bytes(), &creds)
	if err != nil {
		t.Errorf("failed to unmarshal response body: %v", err)
	}

	if creds.AccessKeyId == "" {
		t.Errorf("access key is empty")
	}

	if creds.SecretAccessKey == "" {
		t.Errorf("secret key is empty")
	}

}
