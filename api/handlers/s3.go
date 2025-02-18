package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/EO-DataHub/eodhp-workspace-services/api/middleware"
	"github.com/EO-DataHub/eodhp-workspace-services/internal/authn"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/gorilla/mux"
	_ "github.com/lib/pq"
	"github.com/rs/zerolog"
)

const TimeFormat string = "2006-01-02T15:04:05Z"

type STSClient interface {
	AssumeRoleWithWebIdentity(ctx context.Context,
		params *sts.AssumeRoleWithWebIdentityInput, optFns ...func(*sts.Options)) (
		*sts.AssumeRoleWithWebIdentityOutput, error)
}

// Request s3 credentials for assumed role
func RequestS3Credentials(roleArn string, c STSClient) http.HandlerFunc {

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

		claims, ok := r.Context().Value(middleware.ClaimsKey).(authn.Claims)
		if !ok {
			err := "Invalid claims"
			http.Error(w, err, http.StatusUnauthorized)
			logger.Error().Msg(err)
			return
		}

		logger = logger.With().Str("claims user", claims.Username).Logger()

		if tokenExchangeRequired(claims, workspaceID, userID) {
			logger.Info().Msg("Token exchange required")
			// TODO: Exchange token for workspace scoped token
			http.Error(w, "Token exchange not currently supported",
				http.StatusBadRequest) // Temporary guard until TODO implemented
			return
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
	SecretAccessKey string `json:"secret"`
	SessionToken    string `json:"sessionToken"`
	Expiration      string `json:"expiration"`
}

func tokenExchangeRequired(claims authn.Claims, workspaceID string,
	userID string) bool {

	return workspaceID != claims.Workspace || userID != claims.Username
}
