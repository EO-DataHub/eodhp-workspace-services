package handlers

import (
	"errors"
	"net/http"

	"github.com/EO-DataHub/eodhp-workspace-services/api/middleware"
	"github.com/EO-DataHub/eodhp-workspace-services/db"
	"github.com/EO-DataHub/eodhp-workspace-services/internal/authn"
	services "github.com/EO-DataHub/eodhp-workspace-services/internal/services"
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

		claims, ok := r.Context().Value(middleware.ClaimsKey).(authn.Claims)
		if !ok {
			services.HandleErrResponse(w, http.StatusUnauthorized, errors.New("unauthorized: invalid claims"))
			return
		}

		// Check if the user is an admin
		if !hasRole(claims, "hub_admin") {
			services.HandleErrResponse(w, http.StatusForbidden, errors.New("forbidden: administrator use only"))
			return
		}

		services.UpdateAccountService(workspaceDB, w, r)
	}
}

func hasRole(claims authn.Claims, role string) bool {
	// Check if the user has the required role in the realm_access.roles field
	if len(claims.RealmAccess.Roles) > 0 {
		for _, r := range claims.RealmAccess.Roles {
			if r == role {
				return true
			}
		}
	}
	return false
}
