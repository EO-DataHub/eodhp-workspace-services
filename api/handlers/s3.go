package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/EO-DataHub/eodhp-workspace-services/api/middleware"
	"github.com/EO-DataHub/eodhp-workspace-services/aws"
	"github.com/EO-DataHub/eodhp-workspace-services/internal/authn"
	_ "github.com/lib/pq"
	"github.com/rs/zerolog"
)

// GetS3Credentials handles HTTP requests for retreiving s3 STS credentials from AWS
func GetS3Credentials(stsClient aws.STSClient) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {
		logger := zerolog.Ctx(r.Context())

		// Get the username name from the claims
		// TODO: replace username with the workspace name

		claims, ok := r.Context().Value(middleware.ClaimsKey).(authn.Claims)

		if !ok {
			http.Error(w, "Invalid claims or missing authorization token", http.StatusUnauthorized)
			logger.Error().Msg("Invalid claims or missing authorization")
			return
		}

		workspaceName := claims.Username

		creds, err := stsClient.AssumeRoleWithWebIdentity(workspaceName)
		if err != nil {
			logger.Err(err).Msg("Failed to retrieve S3 credentials")
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(creds)

		logger.Info().Msg("S3 credentials retrieved")

	}

}
