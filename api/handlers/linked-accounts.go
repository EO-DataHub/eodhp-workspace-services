package handlers

import (
	"net/http"

	services "github.com/EO-DataHub/eodhp-workspace-services/api/services"
)

// CreateAccount handles HTTP requests for creating a new account.
func CreateLinkedAccount(svc *services.Service) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {

		services.CreateLinkedAccountService(svc, w, r)
	}
}
