package services

import (
	"encoding/json"
	"errors"

	"net/http"

	"github.com/EO-DataHub/eodhp-workspace-services/api/middleware"
	"github.com/EO-DataHub/eodhp-workspace-services/db"
	"github.com/EO-DataHub/eodhp-workspace-services/internal/authn"
	"github.com/EO-DataHub/eodhp-workspace-services/models"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
)

// Handles the creation of a workspace and its related components
func CreateAccountService(workspaceDB *db.WorkspaceDB, w http.ResponseWriter, r *http.Request) {

	claims, ok := r.Context().Value(middleware.ClaimsKey).(authn.Claims)
	if !ok {
		http.Error(w, "Unauthorized: invalid claims", http.StatusUnauthorized)
		return
	}

	var messagePayload models.Account
	if err := json.NewDecoder(r.Body).Decode(&messagePayload); err != nil {
		workspaceDB.Log.Error().Err(err).Msg("Invalid request payload")
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	// Default owner is the user
	messagePayload.AccountOwner = claims.Username

	account, err := workspaceDB.InsertAccount(&messagePayload)
	if err != nil {
		HandleErrResponse(w, http.StatusInternalServerError, err)
	}

	HandleSuccessResponse(w, http.StatusCreated, nil, models.Response{
		Success: 1,
		Data:    models.AccountResponse{Account: *account},
	})
}

func GetAccountsService(workspaceDB *db.WorkspaceDB, w http.ResponseWriter, r *http.Request) {

	// Get the claims from the context
	claims, ok := r.Context().Value(middleware.ClaimsKey).(authn.Claims)
	if !ok {
		http.Error(w, "Unauthorized: invalid claims", http.StatusUnauthorized)
		return
	}

	// Retrieve user workspaces based on the username
	accounts, err := workspaceDB.GetAccounts(claims.Username)

	if err != nil {
		HandleErrResponse(w, http.StatusInternalServerError, err)
		return
	}

	HandleSuccessResponse(w, http.StatusOK, nil, models.Response{
		Success: 1,
		Data:    models.AccountsResponse{Accounts: accounts},
	})

}

func UpdateAccountService(workspaceDB *db.WorkspaceDB, w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)

	accountID, err := uuid.Parse(vars["account-id"])

	if err != nil {
		HandleErrResponse(w, http.StatusBadRequest, err)
		return
	}

	var updatePayload struct {
		AccountOwner string `json:"accountOwner"`
	}
	if err := json.NewDecoder(r.Body).Decode(&updatePayload); err != nil {
		HandleErrResponse(w, http.StatusBadRequest, err)
		return
	}
	if updatePayload.AccountOwner == "" {
		HandleErrResponse(w, http.StatusBadRequest, errors.New("accountOwner cannot be empty"))
		return
	}

	// Call the UpdateAccountOwner function with the new owner
	updatedAccount, err := workspaceDB.UpdateAccountOwner(accountID, updatePayload.AccountOwner)
	if err != nil {
		HandleErrResponse(w, http.StatusInternalServerError, err)
		return
	}

	// Send a success response with the updated account data
	HandleSuccessResponse(w, http.StatusOK, nil, models.Response{
		Success: 1,
		Data:    models.AccountResponse{Account: *updatedAccount},
	})

}

func DeleteAccountService(workspaceDB *db.WorkspaceDB, w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)

	accountID, err := uuid.Parse(vars["account-id"])
	if err != nil {
		http.Error(w, "Invalid account ID", http.StatusBadRequest)
		return
	}

	err = workspaceDB.DeleteAccount(accountID)

	if err != nil {
		HandleErrResponse(w, http.StatusInternalServerError, err)
		return
	}

	HandleSuccessResponse(w, http.StatusOK, nil, models.Response{
		Success: 1,
	})

}
