package services

import (
	"encoding/json"
	"fmt"

	"net/http"

	"github.com/EO-DataHub/eodhp-workspace-services/api/middleware"
	"github.com/EO-DataHub/eodhp-workspace-services/internal/authn"
	ws_services "github.com/EO-DataHub/eodhp-workspace-services/models"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
)

// CreateAccountService creates a new account for the authenticated user.
func CreateAccountService(svc *Service, w http.ResponseWriter, r *http.Request) {

	// Retrieve claims from the request context to identify the user
	claims, ok := r.Context().Value(middleware.ClaimsKey).(authn.Claims)
	if !ok {
		WriteResponse(w, http.StatusUnauthorized, nil)
		return
	}

	// Decode the request payload into an Account struct
	var messagePayload ws_services.Account
	if err := json.NewDecoder(r.Body).Decode(&messagePayload); err != nil {
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
		WriteResponse(w, http.StatusInternalServerError, nil)
		return
	}

	// Add location header
	var location = fmt.Sprintf("%s/%s", r.URL.Path, account.ID)

	// Send a success response with the created account data
	WriteResponse(w, http.StatusCreated, *account, location)

}

// GetAccountsService retrieves all accounts for the authenticated user.
func GetAccountsService(svc *Service, w http.ResponseWriter, r *http.Request) {

	// Extract claims from the request context to identify the user
	claims, ok := r.Context().Value(middleware.ClaimsKey).(authn.Claims)
	if !ok {
		WriteResponse(w, http.StatusUnauthorized, nil)
		return
	}

	// Retrieve accounts associated with the user's username
	accounts, err := svc.DB.GetAccounts(claims.Username)

	if err != nil {
		WriteResponse(w, http.StatusInternalServerError, nil)
		return
	}

	// Send a success response with the retrieved accounts data
	WriteResponse(w, http.StatusOK, accounts)

}

// GetAccountService retrieves a single account all accounts for the authenticated user.
func GetAccountService(svc *Service, w http.ResponseWriter, r *http.Request) {

	// Extract claims from the request context to identify the user
	claims, ok := r.Context().Value(middleware.ClaimsKey).(authn.Claims)
	if !ok {
		WriteResponse(w, http.StatusUnauthorized, nil)
		return
	}

	// Parse the account ID from the URL path
	accountID, err := uuid.Parse(mux.Vars(r)["account-id"])

	if err != nil {
		WriteResponse(w, http.StatusBadRequest, nil)
		return
	}

	// Retrieve account associated with the user's username
	account, err := svc.DB.GetAccount(accountID)

	if err != nil {
		WriteResponse(w, http.StatusInternalServerError, nil)
		return
	}

	// Handle non-existent account
	if account == nil {
		WriteResponse(w, http.StatusNotFound, nil)
		return
	}

	// Check if the account owner matches the claims username
	if account.AccountOwner != claims.Username {
		WriteResponse(w, http.StatusForbidden, nil)
		return
	}

	// Send a success response with the retrieved accounts data
	WriteResponse(w, http.StatusOK, *account)

}

// UpdateAccountService updates an account based on account ID from the URL path.
func UpdateAccountService(svc *Service, w http.ResponseWriter, r *http.Request) {

	// Parse the account ID from the URL path
	accountID, err := uuid.Parse(mux.Vars(r)["account-id"])

	if err != nil {
		WriteResponse(w, http.StatusBadRequest, nil)
		return
	}

	// Decode the request payload into an Account struct
	var updatePayload ws_services.Account
	if err := json.NewDecoder(r.Body).Decode(&updatePayload); err != nil {
		WriteResponse(w, http.StatusBadRequest, nil)
		return
	}

	// Call UpdateAccount to change the account fields in the database
	updatedAccount, err := svc.DB.UpdateAccount(accountID, updatePayload)
	if err != nil {
		WriteResponse(w, http.StatusInternalServerError, nil)
		return
	}

	// Send a success response with the updated account data
	WriteResponse(w, http.StatusOK, *updatedAccount)

}

// DeleteAccountService deletes an account specified by the account ID from the URL path.
func DeleteAccountService(svc *Service, w http.ResponseWriter, r *http.Request) {

	accountID, err := uuid.Parse(mux.Vars(r)["account-id"])
	if err != nil {
		http.Error(w, "Invalid account ID", http.StatusBadRequest)
		return
	}

	// TODO: Need to send a publish message to delete all workspaces associated with the account
	err = svc.DB.DeleteAccount(accountID)

	if err != nil {
		WriteResponse(w, http.StatusInternalServerError, nil)
		return
	}

	WriteResponse(w, http.StatusNoContent, nil)
}
