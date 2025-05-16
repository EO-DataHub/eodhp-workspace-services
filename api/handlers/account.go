package handlers

import (
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

// GetAccounts retrieves all billing accounts for the authenticated user.
// @Summary Get billing accounts
// @Description Retrieve a list of billing accounts owned by the authenticated user.
// @Tags Billing and Billing Accounts
// @Accept json
// @Produce json
// @Success 200 {array} models.Account
// @Failure 401 {object} string
// @Failure 500 {object} string
// @Router /accounts [get]
func GetAccounts(svc *services.BillingAccountService) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {

		svc.GetAccountsService(w, r)
	}
}

// GetAccount retrieves a billing account by ID.
// @Summary Get a billing account
// @Description Retrieve details of a specific billing account by its unique ID.
// @Tags Billing and Billing Accounts
// @Accept json
// @Produce json
// @Param id path string true "Account ID"
// @Success 200 {object} models.Account
// @Failure 400 {object} string
// @Failure 401 {object} string
// @Failure 404 {object} string
// @Failure 500 {object} string
// @Router /accounts/{id} [get]
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
func AccountStatusHandler(svc *services.BillingAccountService, accountStatusRequest string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		// Get a token from keycloak so we can interact with it's API
		err := svc.KC.GetToken()
		if err != nil {
			fmt.Println("Error getting token from Keycloak:", err)
			http.Error(w, "Authentication failed.", http.StatusInternalServerError)
			return
		}

		svc.AccountApprovalService(w, r, accountStatusRequest)

	}
}
