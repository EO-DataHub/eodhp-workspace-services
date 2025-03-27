package awsclient

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/aws/aws-sdk-go-v2/service/sesv2"
	"github.com/aws/aws-sdk-go-v2/service/sts"
)

// LoadAWSConfig initializes and returns an AWS SDK configuration.
func LoadAWSConfig(region string) (aws.Config, error) {
	cfg, err := config.LoadDefaultConfig(context.TODO(), config.WithRegion(region))
	if err != nil {
		return aws.Config{}, fmt.Errorf("unable to load SDK config: %v", err)
	}
	return cfg, nil
}

// NewSecretsManagerClient initializes the AWS Secrets Manager client.
func NewSecretsManagerClient(cfg aws.Config) *secretsmanager.Client {
	return secretsmanager.NewFromConfig(cfg)
}

// NewSESClient initializes the AWS SES client.
func NewSESClient(cfg aws.Config) *sesv2.Client {
	return sesv2.NewFromConfig(cfg)
}

// NewSTSClient initializes the AWS STS client.
func NewSTSClient(cfg aws.Config) *sts.Client {
	return sts.NewFromConfig(cfg)
}

// Initialize S3 Client
func NewS3Client(cfg aws.Config) *s3.Client {
	return s3.NewFromConfig(cfg)
}
