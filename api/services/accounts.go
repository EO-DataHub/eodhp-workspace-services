package services

import (
	"encoding/json"
	"fmt"

	"net/http"

	"github.com/EO-DataHub/eodhp-workspace-services/api/middleware"
	"github.com/EO-DataHub/eodhp-workspace-services/db"
	"github.com/EO-DataHub/eodhp-workspace-services/internal/authn"
	"github.com/EO-DataHub/eodhp-workspace-services/models"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
)

// CreateAccountService creates a new account for the authenticated user.
func CreateAccountService(workspaceDB *db.WorkspaceDB, w http.ResponseWriter, r *http.Request) {

	// Retrieve claims from the request context to identify the user
	claims, ok := r.Context().Value(middleware.ClaimsKey).(authn.Claims)
	if !ok {
		http.Error(w, "Unauthorized: invalid claims", http.StatusUnauthorized)
		return
	}

	// Decode the request payload into an Account struct
	var messagePayload models.Account
	if err := json.NewDecoder(r.Body).Decode(&messagePayload); err != nil {
		HandleErrResponse(workspaceDB, w, http.StatusBadRequest, err)
		return
	}

	// Only hub_admin can create accounts owned by other users otherwise the account owner is the authenticated user
	if !HasRole(claims.RealmAccess.Roles, "hub_admin") {
		messagePayload.AccountOwner = claims.Username
	}

	// Create the account in the database
	account, err := workspaceDB.CreateAccount(&messagePayload)
	if err != nil {
		HandleErrResponse(workspaceDB, w, http.StatusInternalServerError, err)
		return
	}

	// Add location header
	var location = fmt.Sprintf("%s/%s", r.URL.Path, account.ID)

	// Send a success response with the created account data
	HandleSuccessResponse(w, http.StatusCreated, nil, models.Response{
		Success: 1,
		Data:    models.AccountResponse{Account: *account},
	}, location)
}

// GetAccountsService retrieves all accounts for the authenticated user.
func GetAccountsService(workspaceDB *db.WorkspaceDB, w http.ResponseWriter, r *http.Request) {

	// Extract claims from the request context to identify the user
	claims, ok := r.Context().Value(middleware.ClaimsKey).(authn.Claims)
	if !ok {
		http.Error(w, "Unauthorized: invalid claims", http.StatusUnauthorized)
		return
	}

	// Retrieve accounts associated with the user's username
	accounts, err := workspaceDB.GetAccounts(claims.Username)

	if err != nil {
		HandleErrResponse(workspaceDB, w, http.StatusInternalServerError, err)
		return
	}

	// Send a success response with the retrieved accounts data
	HandleSuccessResponse(w, http.StatusOK, nil, models.Response{
		Success: 1,
		Data:    models.AccountsResponse{Accounts: accounts},
	}, "")

}

// GetAccountService retrieves a single account all accounts for the authenticated user.
func GetAccountService(workspaceDB *db.WorkspaceDB, w http.ResponseWriter, r *http.Request) {

	// Extract claims from the request context to identify the user
	claims, ok := r.Context().Value(middleware.ClaimsKey).(authn.Claims)
	if !ok {
		http.Error(w, "Unauthorized: invalid claims", http.StatusUnauthorized)
		return
	}

	// Parse the account ID from the URL path
	accountID, err := uuid.Parse(mux.Vars(r)["account-id"])

	if err != nil {
		HandleErrResponse(workspaceDB, w, http.StatusBadRequest, err)
		return
	}

	// Retrieve account associated with the user's username
	account, err := workspaceDB.GetAccount(accountID)

	if err != nil {
		HandleErrResponse(workspaceDB, w, http.StatusInternalServerError, err)
		return
	}

	// Handle non-existent account
	if account == nil {
		// Return 404 Not Found as if the account does not exist
		HandleSuccessResponse(w, http.StatusNotFound, nil, models.Response{
			Success:      0,
			ErrorCode:    "not_found",
			ErrorDetails: "The requested account does not exist.",
		}, "")
		return
	}

	// Check if the account owner matches the claims username
	if account.AccountOwner != claims.Username {
		// Return a success: 0 response to indicate unauthorized access without exposing details
		HandleSuccessResponse(w, http.StatusForbidden, nil, models.Response{
			Success:      0,
			ErrorCode:    "unauthorized",
			ErrorDetails: "You do not have access to this account.",
		}, "")
		return
	}

	// Send a success response with the retrieved accounts data
	HandleSuccessResponse(w, http.StatusOK, nil, models.Response{
		Success: 1,
		Data:    models.AccountResponse{Account: *account},
	}, "")

}

// UpdateAccountService updates an account based on account ID from the URL path.
func UpdateAccountService(workspaceDB *db.WorkspaceDB, w http.ResponseWriter, r *http.Request) {

	// Parse the account ID from the URL path
	accountID, err := uuid.Parse(mux.Vars(r)["account-id"])

	if err != nil {
		HandleErrResponse(workspaceDB, w, http.StatusBadRequest, err)
		return
	}

	// Decode the request payload into an Account struct
	var updatePayload models.Account
	if err := json.NewDecoder(r.Body).Decode(&updatePayload); err != nil {
		HandleErrResponse(workspaceDB, w, http.StatusBadRequest, err)
		return
	}

	// Call UpdateAccount to change the account fields in the database
	updatedAccount, err := workspaceDB.UpdateAccount(accountID, updatePayload)
	if err != nil {
		HandleErrResponse(workspaceDB, w, http.StatusInternalServerError, err)
		return
	}

	// Send a success response with the updated account data
	HandleSuccessResponse(w, http.StatusOK, nil, models.Response{
		Success: 1,
		Data:    models.AccountResponse{Account: *updatedAccount},
	}, "")

}

// DeleteAccountService deletes an account specified by the account ID from the URL path.
func DeleteAccountService(workspaceDB *db.WorkspaceDB, w http.ResponseWriter, r *http.Request) {

	accountID, err := uuid.Parse(mux.Vars(r)["account-id"])
	if err != nil {
		http.Error(w, "Invalid account ID", http.StatusBadRequest)
		return
	}

	// TODO: Need to send a publish message to delete all workspaces associated with the account
	err = workspaceDB.DeleteAccount(accountID)

	if err != nil {
		HandleErrResponse(workspaceDB, w, http.StatusInternalServerError, err)
		return
	}

	HandleSuccessResponse(w, http.StatusOK, nil, models.Response{
		Success: 1,
	}, "")

}
