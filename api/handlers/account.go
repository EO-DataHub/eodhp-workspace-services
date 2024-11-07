package handlers

import (
	"net/http"

	"github.com/EO-DataHub/eodhp-workspace-services/db"
	"github.com/EO-DataHub/eodhp-workspace-services/internal/services"
	_ "github.com/lib/pq"
)

func CreateAccount(workspaceDB *db.WorkspaceDB) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {

		services.CreateAccountService(workspaceDB, w, r)
	}
}

func GetAccounts(workspaceDB *db.WorkspaceDB) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {

		services.GetAccountsService(workspaceDB, w, r)
	}
}

func DeleteAccount(workspaceDB *db.WorkspaceDB) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {

		services.DeleteAccountService(workspaceDB, w, r)
	}
}

func UpdateAccount(workspaceDB *db.WorkspaceDB) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {

		services.UpdateAccountService(workspaceDB, w, r)
	}
}
