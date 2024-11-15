package aws

import (
	"context"
	"os"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/aws/aws-sdk-go/aws"
)

type STSClient interface {
	AssumeRoleWithWebIdentity(workspaceName string) (*S3STSCredentialsResponse, error)
}

// Default implementation of STSClient to call AWS STS
type DefaultSTSClient struct{}

func (c *DefaultSTSClient) AssumeRoleWithWebIdentity(workspaceName string) (*S3STSCredentialsResponse, error) {
	return AssumeRoleWithWebIdentity(workspaceName)
}

type S3STSCredentialsResponse struct {
	AccessKeyId     string `json:"accessKeyId"`
	SecretAccessKey string `json:"secretAccessKey"`
	SessionToken    string `json:"sessionToken"`
	Expiration      string `json:"expiration"`
}

func AssumeRoleWithWebIdentity(workspaceName string) (*S3STSCredentialsResponse, error) {

	// We are loading these from the service account
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		return nil, err
	}

	// Create STS client
	svc := sts.NewFromConfig(cfg)

	// Assume role
	input := &sts.AssumeRoleInput{
		RoleArn:         aws.String(os.Getenv("WS_ROLE_ARN_PREFIX") + workspaceName),
		RoleSessionName: aws.String("WorkspaceSession"),
	}

	result, err := svc.AssumeRole(context.TODO(), input)
	if err != nil {
		return nil, err
	}

	// Return the credentials as a response struct
	return &S3STSCredentialsResponse{
		AccessKeyId:     *result.Credentials.AccessKeyId,
		SecretAccessKey: *result.Credentials.SecretAccessKey,
		SessionToken:    *result.Credentials.SessionToken,
		Expiration:      result.Credentials.Expiration.String(),
	}, nil

}
