package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"

	services "github.com/EO-DataHub/eodhp-workspace-services/api/services"
	_ "github.com/lib/pq"
)

// CreateAccount handles HTTP requests for creating a new account.
func CreateAccount(svc *services.BillingAccountService) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {

		svc.CreateAccountService(w, r)
	}
}

// GetAccounts handles HTTP requests for retrieving accounts.
func GetAccounts(svc *services.BillingAccountService) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {

		svc.GetAccountsService(w, r)
	}
}

// GetAccount handles HTTP requests for retrieving a single account.
func GetAccount(svc *services.BillingAccountService) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {

		svc.GetAccountService(w, r)
	}
}

// DeleteAccount handles HTTP requests for deleting an account.
func DeleteAccount(svc *services.BillingAccountService) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {

		svc.DeleteAccountService(w, r)
	}
}

// UpdateAccount handles HTTP requests for updating an account.
func UpdateAccount(svc *services.BillingAccountService) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {

		svc.UpdateAccountService(w, r)
	}
}

// ApproveAccount handles account approval requests
func AccountStatusRequests(svc *services.BillingAccountService, accountStatusRequest string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		// Get token from query params
		token := r.URL.Query().Get("token")
		if token == "" {
			http.Error(w, "Token is required", http.StatusBadRequest)
			return
		}

		// Validate token
		accountID, err := svc.DB.ValidateApprovalToken(token)
		if err != nil {
			http.Error(w, "Invalid or expired token", http.StatusBadRequest)
			return
		}

		if err := svc.DB.UpdateAccountStatus(accountID, accountStatusRequest); err != nil {
			http.Error(w, "Failed to approve account", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(fmt.Sprintf("Account has been %s", accountStatusRequest))
	}
}
