package aws

import (
    "context"
    "github.com/aws/aws-sdk-go-v2/aws"
    "github.com/aws/aws-sdk-go-v2/config"
    "github.com/aws/aws-sdk-go-v2/service/sts"
)

type CredentialsResponse struct {
    AccessKeyId     string `json:"accessKeyId"`
    SecretAccessKey string `json:"secretAccessKey"`
    SessionToken    string `json:"sessionToken"`
    Expiration      string `json:"expiration"`
}

func AssumeRole(roleArn string) (*CredentialsResponse, error) {
    // Load the Shared AWS Configuration (~/.aws/config)
    cfg, err := config.LoadDefaultConfig(context.TODO(), config.WithRegion("us-west-2"))
    if err != nil {
        return nil, err
    }

    // Create STS client
    svc := sts.NewFromConfig(cfg)

    // Assume role
    input := &sts.AssumeRoleInput{
        RoleArn:         aws.String(roleArn),
        RoleSessionName: aws.String("WorkspaceSession"),
    }

    result, err := svc.AssumeRole(context.TODO(), input)
    if err != nil {
        return nil, err
    }

    // Return the credentials as a response struct
    return &CredentialsResponse{
        AccessKeyId:     *result.Credentials.AccessKeyId,
        SecretAccessKey: *result.Credentials.SecretAccessKey,
        SessionToken:    *result.Credentials.SessionToken,
        Expiration:      result.Credentials.Expiration.String(),
    }, nil
}