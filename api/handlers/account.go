package handlers

import (
	"net/http"

	services "github.com/EO-DataHub/eodhp-workspace-services/api/services"
	"github.com/EO-DataHub/eodhp-workspace-services/db"
	_ "github.com/lib/pq"
)

// CreateAccount handles HTTP requests for creating a new account.
func CreateAccount(workspaceDB *db.WorkspaceDB) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {

		services.CreateAccountService(workspaceDB, w, r)
	}
}

// GetAccounts handles HTTP requests for retrieving accounts.
func GetAccounts(workspaceDB *db.WorkspaceDB) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {

		services.GetAccountsService(workspaceDB, w, r)
	}
}

// GetAccount handles HTTP requests for retrieving a single account.
func GetAccount(workspaceDB *db.WorkspaceDB) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {

		services.GetAccountService(workspaceDB, w, r)
	}
}

// DeleteAccount handles HTTP requests for deleting an account.
func DeleteAccount(workspaceDB *db.WorkspaceDB) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {

		services.DeleteAccountService(workspaceDB, w, r)
	}
}

// UpdateAccount handles HTTP requests for updating an account.
func UpdateAccount(workspaceDB *db.WorkspaceDB) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {

		services.UpdateAccountService(workspaceDB, w, r)
	}
}
