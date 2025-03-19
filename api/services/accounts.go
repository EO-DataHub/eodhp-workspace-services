package services

import (
	"encoding/json"
	"fmt"

	"net/http"

	"github.com/EO-DataHub/eodhp-workspace-services/api/middleware"
	"github.com/EO-DataHub/eodhp-workspace-services/internal/authn"
	"github.com/EO-DataHub/eodhp-workspace-services/models"
	ws_services "github.com/EO-DataHub/eodhp-workspace-services/models"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/rs/zerolog"
)

// CreateAccountService creates a new account for the authenticated user.
func CreateAccountService(svc *Service, w http.ResponseWriter, r *http.Request) {

	logger := zerolog.Ctx(r.Context())

	// Retrieve claims from the request context to identify the user
	claims, ok := r.Context().Value(middleware.ClaimsKey).(authn.Claims)
	if !ok {
		logger.Warn().Msg("Unauthorized request: missing claims")
		WriteResponse(w, http.StatusUnauthorized, nil)
		return
	}

	// Decode the request payload into an Account struct
	var messagePayload ws_services.Account
	if err := json.NewDecoder(r.Body).Decode(&messagePayload); err != nil {
		logger.Warn().Err(err).Msg("Invalid request payload")
		WriteResponse(w, http.StatusBadRequest, nil)
		return
	}

	// Only hub_admin can create accounts owned by other users otherwise the account owner is the authenticated user
	if !HasRole(claims.RealmAccess.Roles, "hub_admin") {
		messagePayload.AccountOwner = claims.Username
	}

	// Create the account in the database
	account, err := svc.DB.CreateAccount(&messagePayload)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to create account in database")
		WriteResponse(w, http.StatusInternalServerError, nil)
		return
	}

	logger.Info().Str("account_id", account.ID.String()).Msg("Account created successfully")

	// Send response
	var location = fmt.Sprintf("%s/%s", r.URL.Path, account.ID)
	WriteResponse(w, http.StatusCreated, *account, location)

}

// GetAccountsService retrieves all accounts for the authenticated user.
func GetAccountsService(svc *Service, w http.ResponseWriter, r *http.Request) {

	logger := zerolog.Ctx(r.Context())

	// Extract claims from the request context to identify the user
	claims, ok := r.Context().Value(middleware.ClaimsKey).(authn.Claims)
	if !ok {
		logger.Warn().Msg("Unauthorized request: missing claims")
		WriteResponse(w, http.StatusUnauthorized, nil)
		return
	}

	// Retrieve accounts associated with the user's username
	accounts, err := svc.DB.GetAccounts(claims.Username)

	if err != nil {
		logger.Error().Err(err).Msg("Failed to retrieve accounts from database")
		WriteResponse(w, http.StatusInternalServerError, nil)
		return
	}

	// Ensure accounts is not nil, return an empty slice if no accounts are found
	if accounts == nil {
		accounts = []models.Account{}
	}

	logger.Info().Int("account_count", len(accounts)).Msg("Successfully retrieved accounts")
	WriteResponse(w, http.StatusOK, accounts)

}

// GetAccountService retrieves a single account all accounts for the authenticated user.
func GetAccountService(svc *Service, w http.ResponseWriter, r *http.Request) {

	logger := zerolog.Ctx(r.Context())

	// Extract claims from the request context to identify the user
	claims, ok := r.Context().Value(middleware.ClaimsKey).(authn.Claims)
	if !ok {
		logger.Warn().Msg("Unauthorized request: missing claims")
		WriteResponse(w, http.StatusUnauthorized, nil)
		return
	}

	// Parse the account ID from the URL path
	accountID, err := uuid.Parse(mux.Vars(r)["account-id"])
	if err != nil {
		logger.Error().Err(err).Msg("Account doesn't exist")
		WriteResponse(w, http.StatusBadRequest, nil)
		return
	}

	// Retrieve account associated with the user's username
	account, err := svc.DB.GetAccount(accountID)

	if err != nil {
		logger.Error().Err(err).Str("account_id", accountID.String()).Msg("Database error retrieving account")
		WriteResponse(w, http.StatusInternalServerError, nil)
		return
	}

	// Handle non-existent account
	if account == nil {
		logger.Warn().Str("account_id", accountID.String()).Msg("Account not found")
		WriteResponse(w, http.StatusNotFound, nil)
		return
	}

	// Check if the account owner matches the claims username
	if account.AccountOwner != claims.Username {
		logger.Warn().Str("account_id", accountID.String()).Str("requested_by", claims.Username).Msg("Access denied: User not owner of account")
		WriteResponse(w, http.StatusForbidden, nil)
		return
	}

	logger.Info().Str("account_id", account.ID.String()).Msg("Successfully retrieved account")
	WriteResponse(w, http.StatusOK, *account)

}

// UpdateAccountService updates an account based on account ID from the URL path.
func UpdateAccountService(svc *Service, w http.ResponseWriter, r *http.Request) {

	logger := zerolog.Ctx(r.Context())

	// Parse the account ID from the URL path
	accountID, err := uuid.Parse(mux.Vars(r)["account-id"])

	if err != nil {
		logger.Warn().Err(err).Msg("Account doesn't exist")
		WriteResponse(w, http.StatusBadRequest, nil)
		return
	}

	// Decode the request payload into an Account struct
	var updatePayload ws_services.Account
	if err := json.NewDecoder(r.Body).Decode(&updatePayload); err != nil {
		logger.Warn().Err(err).Msg("Invalid update request payload")
		WriteResponse(w, http.StatusBadRequest, nil)
		return
	}

	// Call UpdateAccount to change the account fields in the database
	updatedAccount, err := svc.DB.UpdateAccount(accountID, updatePayload)
	if err != nil {
		logger.Error().Err(err).Str("account_id", accountID.String()).Msg("Database error updating account")
		WriteResponse(w, http.StatusInternalServerError, nil)
		return
	}

	logger.Info().Str("account_id", updatedAccount.ID.String()).Msg("Account updated successfully")
	WriteResponse(w, http.StatusOK, *updatedAccount)

}

// DeleteAccountService deletes an account specified by the account ID from the URL path.
func DeleteAccountService(svc *Service, w http.ResponseWriter, r *http.Request) {

	logger := zerolog.Ctx(r.Context())

	accountID, err := uuid.Parse(mux.Vars(r)["account-id"])
	if err != nil {
		logger.Warn().Err(err).Msg("Account doesn't exist")
		WriteResponse(w, http.StatusBadRequest, nil)
		return
	}

	// TODO: Need to send a publish message to delete all workspaces associated with the account
	err = svc.DB.DeleteAccount(accountID)

	if err != nil {
		logger.Error().Err(err).Str("account_id", accountID.String()).Msg("Database error deleting account")
		WriteResponse(w, http.StatusInternalServerError, nil)
		return
	}

	logger.Info().Str("account_id", accountID.String()).Msg("Account deleted successfully")
	WriteResponse(w, http.StatusNoContent, nil)
}
