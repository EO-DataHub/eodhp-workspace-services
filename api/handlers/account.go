package handlers

import (
	"net/http"

	services "github.com/EO-DataHub/eodhp-workspace-services/api/services"
	_ "github.com/lib/pq"
)

// CreateAccount handles HTTP requests for creating a new account.
func CreateAccount(svc *services.Service) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {

		services.CreateAccountService(svc, w, r)
	}
}

// GetAccounts handles HTTP requests for retrieving accounts.
func GetAccounts(svc *services.Service) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {

		services.GetAccountsService(svc, w, r)
	}
}

// GetAccount handles HTTP requests for retrieving a single account.
func GetAccount(svc *services.Service) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {

		services.GetAccountService(svc, w, r)
	}
}

// DeleteAccount handles HTTP requests for deleting an account.
func DeleteAccount(svc *services.Service) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {

		services.DeleteAccountService(svc, w, r)
	}
}

// UpdateAccount handles HTTP requests for updating an account.
func UpdateAccount(svc *services.Service) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {

		services.UpdateAccountService(svc, w, r)
	}
}
