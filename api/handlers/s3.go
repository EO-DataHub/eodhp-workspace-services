package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/EO-DataHub/eodhp-workspace-services/api/middleware"
	"github.com/EO-DataHub/eodhp-workspace-services/api/services"
	"github.com/EO-DataHub/eodhp-workspace-services/internal/authn"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/gorilla/mux"
	_ "github.com/lib/pq"
	"github.com/rs/zerolog"
)

type STSClient interface {
	AssumeRoleWithWebIdentity(ctx context.Context,
		params *sts.AssumeRoleWithWebIdentityInput, optFns ...func(*sts.Options)) (
		*sts.AssumeRoleWithWebIdentityOutput, error)
}

// GetS3Credentials extracts the core logic to retrieve S3 credentials
func GetS3Credentials(roleArn string, c STSClient, k services.KeycloakClient, r *http.Request) (S3Credentials, error) {
	vars := mux.Vars(r)
	workspaceID := vars["workspace-id"]
	userID := vars["user-id"]

	logger := zerolog.Ctx(r.Context()).With().Str("workspace", workspaceID).
		Str("user", userID).Str("role arn", roleArn).Logger()

	token, ok := r.Context().Value(middleware.TokenKey).(string)
	if !ok {
		err := fmt.Errorf("invalid token")
		logger.Error().Msg(err.Error())
		return S3Credentials{}, err
	}

	logger.Debug().Str("token", token).Msg("Token retrieved")

	claims, ok := r.Context().Value(middleware.ClaimsKey).(authn.Claims)
	if !ok {
		err := fmt.Errorf("invalid claims")
		logger.Error().Msg(err.Error())
		return S3Credentials{}, err
	}

	logger = logger.With().Str("claims user", claims.Username).Logger()

	if userID == "me" {
		userID = claims.Username
	}

	if tokenExchangeRequired(claims, workspaceID, userID) {
		logger.Info().Msg("Token exchange required")

		workspaceToken, err := k.ExchangeToken(token, fmt.Sprintf("workspace:%s", workspaceID))
		if err != nil {
			logger.Error().Err(err).Msg("Failed to get offline token")
			return S3Credentials{}, err
		}
		token = workspaceToken.Access
	}

	resp, err := c.AssumeRoleWithWebIdentity(r.Context(), &sts.AssumeRoleWithWebIdentityInput{
		RoleArn:          &roleArn,
		WebIdentityToken: &token,
		RoleSessionName:  aws.String(fmt.Sprintf("%s-%s", workspaceID, userID)),
	})
	if err != nil {
		logger.Err(err).Msg("Failed to retrieve S3 credentials")
		return S3Credentials{}, err
	}

	return S3Credentials{
		AccessKeyId:     *resp.Credentials.AccessKeyId,
		SecretAccessKey: *resp.Credentials.SecretAccessKey,
		SessionToken:    *resp.Credentials.SessionToken,
		Expiration:      resp.Credentials.Expiration.UTC().Format(TimeFormat),
	}, nil
}

// @Summary Request S3 session credentials
// @Description Request S3 session credentials for user access to a single workspace. {user-id} can be set to "me" to use the token owner's user id.
// @Tags Workspace Manangement
// @Accept json
// @Produce json
// @Param workspace-id path string true "Workspace ID" example(my-workspace)
// @Param user-id path string true "User ID" example(me)
// @Success 200 {object} S3Credentials
// @Failure 400 {object} string
// @Failure 401 {object} string
// @Failure 500 {object} string
// @Router /workspaces/{workspace-id}/{user-id}/s3-tokens [post]
func RequestS3CredentialsHandler(roleArn string, c STSClient, k services.KeycloakClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logger := zerolog.Ctx(r.Context())

		creds, err := GetS3Credentials(roleArn, c, k, r)
		if err != nil {
			var status int
			if httpErr, ok := err.(*services.HTTPError); ok {
				status = httpErr.Status
			} else {
				status = http.StatusInternalServerError
			}
			http.Error(w, err.Error(), status)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(creds); err != nil {
			logger.Error().Err(err).Msg("Failed to encode response")
			http.Error(w, "Failed to encode response", http.StatusInternalServerError)
			return
		}
		logger.Info().Msg("S3 credentials retrieved")
	}
}

type S3Credentials struct {
	AccessKeyId     string `json:"accessKeyId"`
	SecretAccessKey string `json:"secretAccessKey"`
	SessionToken    string `json:"sessionToken"`
	Expiration      string `json:"expiration"`
}

func tokenExchangeRequired(claims authn.Claims, workspaceID string,
	userID string) bool {

	return workspaceID != claims.Workspace || userID != claims.Username
}
