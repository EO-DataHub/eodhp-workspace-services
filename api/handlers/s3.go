package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/EO-DataHub/eodhp-workspace-services/api/middleware"
	"github.com/EO-DataHub/eodhp-workspace-services/aws"
	"github.com/EO-DataHub/eodhp-workspace-services/internal/authn"
	_ "github.com/lib/pq"
	"github.com/rs/zerolog"
)

func GetS3Credentials() http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {
		logger := zerolog.Ctx(r.Context()).With().
			Str("handler", "GetS3Credentials").Logger()

		logger.Info().Msg("Hello, World!")

		// Get the username name from the claims
		// TODO: replace username with the workspace name
		claims, ok := r.Context().Value(middleware.ClaimsKey).(authn.Claims)
		if !ok {
			logger.Error().Msg("No claims found in context")
			http.Error(w, "No claims found in context", http.StatusUnauthorized)
			return
		}

		fmt.Println("Workspace Claims: ", claims.Username)

		// figure out how to extract workspace name from the request - via cookies?
		//roleArn := "arn:aws:iam::312280911266:role/jl-s3-workspace-access"
		roleArn := "arn:aws:iam::312280911266:role/eodhp-dev-y4jFxoD4-jlangstone-tpzuk"

		creds, err := aws.AssumeRole(roleArn)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(creds)

	}

}
