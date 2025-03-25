package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/EO-DataHub/eodhp-workspace-services/api/middleware"
	services "github.com/EO-DataHub/eodhp-workspace-services/api/services"
	"github.com/EO-DataHub/eodhp-workspace-services/internal/authn"
	"github.com/gorilla/mux"
	"github.com/rs/zerolog"
)

// @Summary Request workspace scoped session credentials
// @Description Request workspace scoped session credentials for user access to a single workspace. {user-id} can be set to "me" to use the token owner's user id.
// @Tags workspace session credentials auth
// @Accept json
// @Produce json
// @Param workspace-id path string true "Workspace ID" example(my-workspace)
// @Param user-id path string true "User ID" example(me)
// @Success 200 {object} AuthSessionResponse
// @Failure 400 {object} string
// @Failure 401 {object} string
// @Failure 403 {object} string
// @Failure 500 {object} string
// @Router /workspaces/{workspace-id}/{user-id}/sessions [post]
func CreateWorkspaceSession(kc KeycloakClient) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		workspaceID := vars["workspace-id"]
		userID := vars["user-id"]

		logger := zerolog.Ctx(r.Context()).With().Str("workspace", workspaceID).
			Logger()

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
			logger.Error().Msg(err)
			http.Error(w, err, http.StatusUnauthorized)
			return
		}

		logger = logger.With().Str("claims user", claims.Username).Logger()

		// Endpoint only currently supports session credentials for the token owner
		if userID != "me" && userID != claims.Username {
			err := "Endpoint does not support session credentials for users other than the token owner"
			logger.Error().Msg(err)
			http.Error(w, err, http.StatusBadRequest)
		}

		resp, err := kc.ExchangeToken(token, fmt.Sprintf("workspace:%s", workspaceID))
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

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		response := AuthSessionResponse{
			Access: resp.Access,
			AccessExpiry: time.Now().Add(time.Duration(resp.ExpiresIn) *
				time.Second).Format(TimeFormat),
			Refresh: resp.Refresh,
			RefreshExpiry: time.Now().Add(time.Duration(resp.RefreshExpiresIn) *
				time.Second).Format(TimeFormat),
			Scope: resp.Scope,
		}

		if err := json.NewEncoder(w).Encode(response); err != nil {
			logger.Error().Err(err).Msg("Failed to encode response")
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	})
}

type AuthSessionResponse struct {
	Access        string `json:"access"`
	AccessExpiry  string `json:"accessExpiry"`
	Refresh       string `json:"refresh"`
	RefreshExpiry string `json:"refreshExpiry"`
	Scope         string `json:"scope"`
}
