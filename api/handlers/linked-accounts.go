package handlers

import (
	"net/http"

	services "github.com/EO-DataHub/eodhp-workspace-services/api/services"
)

// CreateAccount handles HTTP requests for creating a new account.
func CreateLinkedAccount(svc *services.LinkedAccountService) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {

		svc.CreateLinkedAccountService(w, r)
	}
}

// GetAccounts handles HTTP requests for retrieving accounts.
func GetLinkedAccounts(svc *services.LinkedAccountService) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {
		svc.GetLinkedAccounts(w, r)
	}
}

// DeleteAccount handles HTTP requests for deleting an account.
func DeleteLinkedAccount(svc *services.LinkedAccountService) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {

		svc.DeleteLinkedAccountService(w, r)
	}
}

// ValidateAirbusLinkedAccount handles HTTP requests for validating an Airbus linked account.
func ValidateAirbusLinkedAccount(svc *services.LinkedAccountService) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {

		svc.ValidateAirbusLinkedAccountService(w, r)
	}
}

// ValidatePlanetLinkedAccount handles HTTP requests for validating an Planet linked account.
func ValidatePlanetLinkedAccount(svc *services.LinkedAccountService) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {

		svc.ValidatePlanetLinkedAccountService(w, r)
	}
}

