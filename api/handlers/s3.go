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

// @Summary Request S3 session credentials
// @Description Request S3 session credentials for user access to a single workspace. {user-id} can be set to "me" to use the token owner's user id.
// @Tags s3 credentials auth
// @Accept json
// @Produce json
// @Param workspace-id path string true "Workspace ID" example(my-workspace)
// @Param user-id path string true "User ID" example(me)
// @Success 200 {object} S3Credentials
// @Failure 400 {object} string
// @Failure 401 {object} string
// @Failure 500 {object} string
// @Router /workspaces/{workspace-id}/{user-id}/s3-tokens [post]
func RequestS3CredentialsHandler(roleArn string, c STSClient,
	k services.KeycloakClient) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		workspaceID := vars["workspace-id"]
		userID := vars["user-id"]

		logger := zerolog.Ctx(r.Context()).With().Str("workspace", workspaceID).
			Str("user", userID).Str("role arn", roleArn).Logger()

		token, ok := r.Context().Value(middleware.TokenKey).(string)
		if !ok {
			err := "Invalid token"
			http.Error(w, err, http.StatusUnauthorized)
			logger.Error().Msg(err)
			return
		}

		logger.Debug().Str("token", token).Msg("Token retrieved")

		claims, ok := r.Context().Value(middleware.ClaimsKey).(authn.Claims)
		if !ok {
			err := "Invalid claims"
			http.Error(w, err, http.StatusUnauthorized)
			logger.Error().Msg(err)
			return
		}

		logger = logger.With().Str("claims user", claims.Username).Logger()

		if userID == "me" {
			userID = claims.Username
		}

		if tokenExchangeRequired(claims, workspaceID, userID) {
			logger.Info().Msg("Token exchange required")

			// Exchange user scoped token for workspace scoped token
			// TODO: Exchange Token will never change the user the token is for,
			// to do this we need to implement a Keycloak Impersonate User API.
			workspaceToken, err := k.ExchangeToken(token, fmt.Sprintf(
				"workspace:%s", workspaceID))
			if err != nil {
				var errStatus int
				if err, ok := err.(*services.HTTPError); ok {
					errStatus = err.Status
				} else {
					errStatus = http.StatusInternalServerError
				}
				logger.Error().Err(err).Msg("Failed to get offline token")
				http.Error(w, err.Error(), errStatus)
				return
			}

			// Replace user scoped token with workspace scoped token
			token = workspaceToken.Access
		}

		resp, err := c.AssumeRoleWithWebIdentity(r.Context(),
			&sts.AssumeRoleWithWebIdentityInput{
				RoleArn:          &roleArn,
				WebIdentityToken: &token,
				RoleSessionName:  aws.String(fmt.Sprintf("%s-%s", workspaceID, userID)),
			})
		if err != nil {
			logger.Err(err).Msg("Failed to retrieve S3 credentials")
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(S3Credentials{
			AccessKeyId:     *resp.Credentials.AccessKeyId,
			SecretAccessKey: *resp.Credentials.SecretAccessKey,
			SessionToken:    *resp.Credentials.SessionToken,
			Expiration:      resp.Credentials.Expiration.UTC().Format(TimeFormat),
		})
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
