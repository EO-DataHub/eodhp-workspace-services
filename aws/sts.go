package aws

import (
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/rs/zerolog/log"
)

type S3STSCredentialsResponse struct {
	AccessKeyId     string `json:"accessKeyId"`
	SecretAccessKey string `json:"secretAccessKey"`
	SessionToken    string `json:"sessionToken"`
	Expiration      string `json:"expiration"`
}

func AssumeRoleWithWebIdentity() (*S3STSCredentialsResponse, error) {

	sess, err := session.NewSession()
	if err != nil {
		log.Error().Err(err).Msg("Failed to create session")
		return nil, err

	}

	// Retrieve the current credentials from the default credentials chain
	creds, err := sess.Config.Credentials.Get()
	if err != nil {
		log.Error().Err(err).Msg("cannot get credentials")
		return nil, err
	}

	credsResponse := S3STSCredentialsResponse{
		AccessKeyId:     creds.AccessKeyID,
		SecretAccessKey: creds.SecretAccessKey,
		SessionToken:    creds.SessionToken,
	}

	return &credsResponse, nil

}
