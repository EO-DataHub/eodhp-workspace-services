package handlers

import (
	"net/http"

	services "github.com/EO-DataHub/eodhp-workspace-services/api/services"
)

// CreateAccount handles HTTP requests for creating a new account.
func CreateLinkedAccount(svc *services.LinkedAccountService) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {

		// Get a token from keycloak so we can interact with it's API
		if !ensureKeycloakToken(w, svc.KC) {
			return
		}

		svc.CreateLinkedAccountService(w, r)
	}
}

// CreateOpenCosmosSession handles HTTP requests for persisting an Open Cosmos session for a workspace.
// @Summary Store an Open Cosmos OAuth session
// @Description Creates or replaces the Open Cosmos OAuth session shared by the workspace in its Kubernetes namespace.
// @Tags Linked Accounts
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param workspace-id path string true "Workspace ID"
// @Param session body services.OpenCosmosSessionPayload true "Open Cosmos OAuth session"
// @Success 201
// @Failure 400 {object} string
// @Failure 401 {object} string
// @Failure 403 {object} string
// @Failure 500 {object} string
// @Router /workspaces/{workspace-id}/open-cosmos/session [post]
func CreateOpenCosmosSession(svc *services.LinkedAccountService) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {

		// Get a token from keycloak so we can interact with its API
		if !ensureKeycloakToken(w, svc.KC) {
			return
		}

		svc.CreateOpenCosmosSessionService(w, r)
	}
}

// GetAccounts handles HTTP requests for retrieving accounts.
func GetLinkedAccounts(svc *services.LinkedAccountService) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {

		// Get a token from keycloak so we can interact with it's API
		if !ensureKeycloakToken(w, svc.KC) {
			return
		}

		svc.GetLinkedAccounts(w, r)
	}
}

// DeleteAccount handles HTTP requests for deleting an account.
func DeleteLinkedAccount(svc *services.LinkedAccountService) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {

		// Get a token from keycloak so we can interact with it's API
		if !ensureKeycloakToken(w, svc.KC) {
			return
		}

		svc.DeleteLinkedAccountService(w, r)
	}
}

// ValidateAirbusLinkedAccount handles HTTP requests for validating an Airbus linked account.
func ValidateAirbusLinkedAccount(svc *services.LinkedAccountService) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {

		svc.ValidateAirbusLinkedAccountService(w, r)
	}
}

// ValidatePlanetLinkedAccount handles HTTP requests for validating an Planet linked account.
func ValidatePlanetLinkedAccount(svc *services.LinkedAccountService) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {

		svc.ValidatePlanetLinkedAccountService(w, r)
	}
}
