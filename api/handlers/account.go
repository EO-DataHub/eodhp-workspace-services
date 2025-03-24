package handlers

import (
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
