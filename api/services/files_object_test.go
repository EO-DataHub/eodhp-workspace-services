package services

import (
	"context"
	"errors"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	ws_manager "github.com/EO-DataHub/eodhp-workspace-manager/models"
	"github.com/EO-DataHub/eodhp-workspace-services/internal/appconfig"
	awsclient "github.com/EO-DataHub/eodhp-workspace-services/internal/aws"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	ststypes "github.com/aws/aws-sdk-go-v2/service/sts/types"
	"github.com/stretchr/testify/require"
)

func TestGetS3CredentialsStaticKeys(t *testing.T) {
	svc := FileService{
		Config: &appconfig.Config{
			AWS: appconfig.AWSConfig{
				S3: appconfig.S3Config{
					AccessKey: "local-key",
					SecretKey: "local-secret",
				},
			},
		},
	}
	req := httptest.NewRequest(http.MethodGet, "/", nil)

	creds, err := svc.getS3Credentials(req)
	require.NoError(t, err)
	require.Equal(t, awsclient.S3Credentials{
		AccessKeyId:     "local-key",
		SecretAccessKey: "local-secret",
		SessionToken:    "",
		Expiration:      "",
	}, creds)
}

func TestGetS3CredentialsMissingAuthHeader(t *testing.T) {
	svc := FileService{
		Config: &appconfig.Config{
			AWS: appconfig.AWSConfig{
				Region: "us-east-1",
				S3: appconfig.S3Config{
					RoleArn: "arn:aws:iam::123456789012:role/test",
				},
			},
		},
	}
	req := httptest.NewRequest(http.MethodGet, "/", nil)

	_, err := svc.getS3Credentials(req)
	require.EqualError(t, err, "authorization header missing")
}

func TestGetS3CredentialsMissingRoleARN(t *testing.T) {
	svc := FileService{
		Config: &appconfig.Config{
			AWS: appconfig.AWSConfig{
				Region: "us-east-1",
			},
		},
	}
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer token-1")

	_, err := svc.getS3Credentials(req)
	require.EqualError(t, err, "missing AWS role ARN for S3 credentials")
}

func TestGetS3CredentialsSTSFailure(t *testing.T) {
	mockSTS := &mockSTSClient{err: errors.New("sts boom")}
	svc := FileService{
		Config: &appconfig.Config{
			AWS: appconfig.AWSConfig{
				Region: "us-east-1",
				S3: appconfig.S3Config{
					RoleArn: "arn:aws:iam::123456789012:role/test",
				},
			},
		},
		STS: mockSTS,
	}
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer token-1")

	_, err := svc.getS3Credentials(req)
	require.EqualError(t, err, "sts boom")
	require.True(t, mockSTS.called)
}

func TestGetS3CredentialsInvalidSTSResponse(t *testing.T) {
	t.Run("missing credentials", func(t *testing.T) {
		mockSTS := &mockSTSClient{out: &sts.AssumeRoleWithWebIdentityOutput{}}
		svc := testServiceWithSTS(mockSTS)
		req := authRequest()

		_, err := svc.getS3Credentials(req)
		require.EqualError(t, err, "missing credentials from STS response")
	})

	t.Run("missing fields", func(t *testing.T) {
		mockSTS := &mockSTSClient{
			out: &sts.AssumeRoleWithWebIdentityOutput{
				Credentials: &ststypes.Credentials{
					AccessKeyId: aws.String("A"),
				},
			},
		}
		svc := testServiceWithSTS(mockSTS)
		req := authRequest()

		_, err := svc.getS3Credentials(req)
		require.EqualError(t, err, "invalid credentials returned by STS")
	})
}

func TestGetS3CredentialsSuccessFromSTS(t *testing.T) {
	exp := time.Date(2026, 2, 18, 9, 0, 0, 0, time.UTC)
	mockSTS := &mockSTSClient{
		out: &sts.AssumeRoleWithWebIdentityOutput{
			Credentials: &ststypes.Credentials{
				AccessKeyId:     aws.String("AKIA123"),
				SecretAccessKey: aws.String("secret-123"),
				SessionToken:    aws.String("token-123"),
				Expiration:      aws.Time(exp),
			},
		},
	}
	svc := testServiceWithSTS(mockSTS)
	req := authRequest()

	creds, err := svc.getS3Credentials(req)
	require.NoError(t, err)
	require.Equal(t, "AKIA123", creds.AccessKeyId)
	require.Equal(t, "secret-123", creds.SecretAccessKey)
	require.Equal(t, "token-123", creds.SessionToken)
	require.Equal(t, "2026-02-18T09:00:00Z", creds.Expiration)
}

func TestObjectStoreMethodsProvisioningValidation(t *testing.T) {
	svc := FileService{
		Config: &appconfig.Config{
			AWS: appconfig.AWSConfig{
				Region: "us-east-1",
				S3: appconfig.S3Config{
					AccessKey: "local-key",
					SecretKey: "local-secret",
				},
			},
		},
	}
	req := httptest.NewRequest(http.MethodGet, "/", nil)

	_, status, err := svc.listObjectStoreItems(req, nil)
	require.Equal(t, http.StatusBadRequest, status)
	require.EqualError(t, err, "no object store configured")

	_, status, err = svc.listObjectStoreItems(req, []ws_manager.ObjectStore{{Bucket: "", Prefix: "prefix"}})
	require.Equal(t, http.StatusBadRequest, status)
	require.EqualError(t, err, "object store not provisioned")

	_, err = svc.uploadObjectStoreFiles(req, ws_manager.ObjectStore{}, []*multipart.FileHeader{})
	require.EqualError(t, err, "object store not provisioned")

	_, _, err = svc.deleteObjectStoreFiles(req, ws_manager.ObjectStore{}, []string{"a.tif"})
	require.EqualError(t, err, "object store not provisioned")

	_, err = svc.getObjectStoreMetadata(req, ws_manager.ObjectStore{}, "a.tif")
	require.EqualError(t, err, "object store not provisioned")
}

func TestListObjectStoreItemsInvalidPrefixReturnsError(t *testing.T) {
	svc := FileService{
		Config: &appconfig.Config{
			AWS: appconfig.AWSConfig{
				Region: "us-east-1",
				S3: appconfig.S3Config{
					AccessKey: "local-key",
					SecretKey: "local-secret",
				},
			},
		},
	}
	req := httptest.NewRequest(http.MethodGet, "/", nil)

	_, status, err := svc.listObjectStoreItems(req, []ws_manager.ObjectStore{
		{Bucket: "bucket-1", Prefix: "/"},
	})
	require.Equal(t, http.StatusBadRequest, status)
	require.EqualError(t, err, "object prefix is required")
}

func TestObjectStorePathValidationWithoutS3Call(t *testing.T) {
	svc := FileService{
		Config: &appconfig.Config{
			AWS: appconfig.AWSConfig{
				Region: "us-east-1",
				S3: appconfig.S3Config{
					AccessKey: "local-key",
					SecretKey: "local-secret",
				},
			},
		},
	}
	req := httptest.NewRequest(http.MethodPost, "/", nil)

	files := []*multipart.FileHeader{
		{Filename: "bad/name.tif"},
	}
	_, err := svc.uploadObjectStoreFiles(req, ws_manager.ObjectStore{
		Bucket: "bucket-1",
		Prefix: "workspace/ws-1",
	}, files)
	require.EqualError(t, err, "nested paths are not supported")

	deleted, failed, err := svc.deleteObjectStoreFiles(req, ws_manager.ObjectStore{
		Bucket: "bucket-1",
		Prefix: "workspace/ws-1",
	}, []string{"bad/name.tif"})
	require.NoError(t, err)
	require.Empty(t, deleted)
	require.Len(t, failed, 1)
	require.Equal(t, "bad/name.tif", failed[0].FileName)

	_, err = svc.getObjectStoreMetadata(req, ws_manager.ObjectStore{
		Bucket: "bucket-1",
		Prefix: "workspace/ws-1",
	}, "bad/name.tif")
	require.EqualError(t, err, "nested paths are not supported")
}

type mockSTSClient struct {
	out    *sts.AssumeRoleWithWebIdentityOutput
	err    error
	called bool
}

func (m *mockSTSClient) AssumeRoleWithWebIdentity(
	_ context.Context,
	_ *sts.AssumeRoleWithWebIdentityInput,
	_ ...func(*sts.Options),
) (*sts.AssumeRoleWithWebIdentityOutput, error) {
	m.called = true
	return m.out, m.err
}

func testServiceWithSTS(stsClient STSClient) FileService {
	return FileService{
		Config: &appconfig.Config{
			AWS: appconfig.AWSConfig{
				Region: "us-east-1",
				S3: appconfig.S3Config{
					RoleArn: "arn:aws:iam::123456789012:role/test",
				},
			},
		},
		STS: stsClient,
	}
}

func authRequest() *http.Request {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer token-1")
	return req
}
