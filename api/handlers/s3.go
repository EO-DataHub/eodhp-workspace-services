package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/EO-DataHub/eodhp-workspace-services/aws"
	_ "github.com/lib/pq"
	"github.com/rs/zerolog"
)

func GetS3Credentials() http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {
		logger := zerolog.Ctx(r.Context()).With().
			Str("handler", "GetS3Credentials").Logger()

		// Get the username name from the claims
		// TODO: replace username with the workspace name
		// TODO: add Expiry to the credentials
		// TODO: as project progresses, we will need to deal with these claims in a more sophisticated way
		// 			claims, ok := r.Context().Value(middleware.ClaimsKey).(authn.Claims)

		creds, err := aws.AssumeRoleWithWebIdentity()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(creds)

		logger.Info().Msg("S3 credentials retrieved")

	}

}
