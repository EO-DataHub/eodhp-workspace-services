package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/EO-DataHub/eodhp-workspace-services/api/middleware"
	"github.com/EO-DataHub/eodhp-workspace-services/internal/authn"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/aws/aws-sdk-go-v2/service/sts/types"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
)

type MockSTSClient struct {
	response *sts.AssumeRoleWithWebIdentityOutput
}

func (c MockSTSClient) AssumeRoleWithWebIdentity(ctx context.Context,
	params *sts.AssumeRoleWithWebIdentityInput, optFns ...func(*sts.Options)) (
	*sts.AssumeRoleWithWebIdentityOutput, error) {

	return c.response, nil
}

func TestGetS3Credentials(t *testing.T) {
	sts_client := MockSTSClient{
		response: &sts.AssumeRoleWithWebIdentityOutput{
			AssumedRoleUser: &types.AssumedRoleUser{
				Arn:           aws.String("arn:aws:iam::123456789012:role/test-role"),
				AssumedRoleId: aws.String("AROACLKWSDQRAOEXAMPLE:test-role"),
			},
			Credentials: &types.Credentials{
				AccessKeyId:     aws.String("ASgeIAIOSFODNN7EXAMPLE"),
				SecretAccessKey: aws.String("wJalrXUtnFEMI/K7MDENG/bPxRfiCYzEXAMPLEKEY"),
				SessionToken:    aws.String("AQoDYXdzEE0a8ANXXXXXXXXNO1ewxE5TijQyp+IEXAMPLE"),
				Expiration:      aws.Time(time.Date(2025, 2, 18, 16, 57, 23, 0, time.UTC)),
			},
		},
	}

	workspace := "test-workspace"
	user := "test-user"

	r, err := http.NewRequest("POST", fmt.Sprintf("/workspaces/%s/%s/s3-tokens", workspace, user), nil)
	assert.NoError(t, err)

	claims := authn.Claims{Username: user, Workspace: workspace}
	r = mux.SetURLVars(r, map[string]string{"user-id": user, "workspace-id": workspace})
	ctx := context.WithValue(r.Context(), middleware.TokenKey, "access-token")
	ctx = context.WithValue(ctx, middleware.ClaimsKey, claims)

	w := httptest.NewRecorder()
	handler := RequestS3Credentials("arn:aws:iam::123456789012:role/test-role", sts_client)
	handler.ServeHTTP(w, r.WithContext(ctx))

	assert.Equal(t, http.StatusOK, w.Code, "handler returned wrong status code")
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"), "handler returned wrong content type")

	var creds S3Credentials
	err = json.Unmarshal(w.Body.Bytes(), &creds)

	assert.NoError(t, err, "failed to unmarshal response body")
	assert.Equal(t, "ASgeIAIOSFODNN7EXAMPLE", creds.AccessKeyId)
	assert.Equal(t, "wJalrXUtnFEMI/K7MDENG/bPxRfiCYzEXAMPLEKEY", creds.SecretAccessKey)
	assert.Equal(t, "AQoDYXdzEE0a8ANXXXXXXXXNO1ewxE5TijQyp+IEXAMPLE", creds.SessionToken)
	assert.Equal(t, "2025-02-18T16:57:23Z", creds.Expiration)
}
