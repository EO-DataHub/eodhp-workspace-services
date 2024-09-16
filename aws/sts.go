package aws

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/aws/aws-sdk-go/aws"
)

type S3STSCredentialsResponse struct {
	AccessKeyId     string `json:"accessKeyId"`
	SecretAccessKey string `json:"secretAccessKey"`
	SessionToken    string `json:"sessionToken"`
	Expiration      string `json:"expiration"`
}

func AssumeRoleWithWebIdentity() (*S3STSCredentialsResponse, error) {

	// sess, err := session.NewSession()
	// if err != nil {
	// 	log.Error().Err(err).Msg("Failed to create session")
	// 	return nil, err

	// }

	// // Retrieve the current credentials from the default credentials chain
	// creds, err := sess.Config.Credentials.Get()
	// if err != nil {
	// 	log.Error().Err(err).Msg("cannot get credentials")
	// 	return nil, err
	// }

	// credsResponse := S3STSCredentialsResponse{
	// 	AccessKeyId:     creds.AccessKeyID,
	// 	SecretAccessKey: creds.SecretAccessKey,
	// 	SessionToken:    creds.SessionToken,
	// }

	// return &credsResponse, nil

	cfg, err := config.LoadDefaultConfig(context.TODO(), config.WithRegion("us-west-2"))
	if err != nil {
		return nil, err
	}

	// Create STS client
	svc := sts.NewFromConfig(cfg)

	// Assume role
	input := &sts.AssumeRoleInput{
		RoleArn:         aws.String("arn:aws:iam::312280911266:role/eodhp-dev-y4jFxoD4-jlangstone-tpzuk"),
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
