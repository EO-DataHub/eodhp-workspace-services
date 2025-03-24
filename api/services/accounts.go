package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"net/http"

	"github.com/EO-DataHub/eodhp-workspace-services/api/middleware"
	"github.com/EO-DataHub/eodhp-workspace-services/internal/authn"
	"github.com/EO-DataHub/eodhp-workspace-services/models"
	ws_services "github.com/EO-DataHub/eodhp-workspace-services/models"
	"github.com/aws/aws-sdk-go-v2/service/sesv2"
	"github.com/aws/aws-sdk-go-v2/service/sesv2/types"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/rs/zerolog"
)

// CreateAccountService creates a new account for the authenticated user.
func (svc *BillingAccountService) CreateAccountService(w http.ResponseWriter, r *http.Request) {

	logger := zerolog.Ctx(r.Context())

	// Retrieve claims from the request context to identify the user
	claims, ok := r.Context().Value(middleware.ClaimsKey).(authn.Claims)
	if !ok {
		logger.Warn().Msg("Unauthorized request: missing claims")
		WriteResponse(w, http.StatusUnauthorized, nil)
		return
	}

	// Decode the request payload into an Account struct
	var messagePayload ws_services.Account
	if err := json.NewDecoder(r.Body).Decode(&messagePayload); err != nil {
		logger.Warn().Err(err).Msg("Invalid request payload")
		WriteResponse(w, http.StatusBadRequest, nil)
		return
	}

	// Only hub_admin can create accounts owned by other users otherwise the account owner is the authenticated user
	if !HasRole(claims.RealmAccess.Roles, "hub_admin") {
		messagePayload.AccountOwner = claims.Username
	}

	// Create the account in the database
	account, err := svc.DB.CreateAccount(&messagePayload)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to create account in database")
		WriteResponse(w, http.StatusInternalServerError, nil)
		return
	}

	logger.Info().Str("account_id", account.ID.String()).Msg("Account created successfully")

	svc.SendEmailAPI(account)

	// Send response
	var location = fmt.Sprintf("%s/%s", r.URL.Path, account.ID)
	WriteResponse(w, http.StatusCreated, *account, location)

}

// GetAccountsService retrieves all accounts for the authenticated user.
func (svc *BillingAccountService) GetAccountsService(w http.ResponseWriter, r *http.Request) {

	logger := zerolog.Ctx(r.Context())

	// Extract claims from the request context to identify the user
	claims, ok := r.Context().Value(middleware.ClaimsKey).(authn.Claims)
	if !ok {
		logger.Warn().Msg("Unauthorized request: missing claims")
		WriteResponse(w, http.StatusUnauthorized, nil)
		return
	}

	// Retrieve accounts associated with the user's username
	accounts, err := svc.DB.GetAccounts(claims.Username)

	if err != nil {
		logger.Error().Err(err).Msg("Failed to retrieve accounts from database")
		WriteResponse(w, http.StatusInternalServerError, nil)
		return
	}

	// Ensure accounts is not nil, return an empty slice if no accounts are found
	if accounts == nil {
		accounts = []models.Account{}
	}

	logger.Info().Int("account_count", len(accounts)).Msg("Successfully retrieved accounts")

	WriteResponse(w, http.StatusOK, accounts)

}

// GetAccountService retrieves a single account all accounts for the authenticated user.
func (svc *BillingAccountService) GetAccountService(w http.ResponseWriter, r *http.Request) {

	logger := zerolog.Ctx(r.Context())

	// Extract claims from the request context to identify the user
	claims, ok := r.Context().Value(middleware.ClaimsKey).(authn.Claims)
	if !ok {
		logger.Warn().Msg("Unauthorized request: missing claims")
		WriteResponse(w, http.StatusUnauthorized, nil)
		return
	}

	// Parse the account ID from the URL path
	accountID, err := uuid.Parse(mux.Vars(r)["account-id"])
	if err != nil {
		logger.Error().Err(err).Msg("Account doesn't exist")
		WriteResponse(w, http.StatusBadRequest, nil)
		return
	}

	// Retrieve account associated with the user's username
	account, err := svc.DB.GetAccount(accountID)

	if err != nil {
		logger.Error().Err(err).Str("account_id", accountID.String()).Msg("Database error retrieving account")
		WriteResponse(w, http.StatusInternalServerError, nil)
		return
	}

	// Handle non-existent account
	if account == nil {
		logger.Warn().Str("account_id", accountID.String()).Msg("Account not found")
		WriteResponse(w, http.StatusNotFound, nil)
		return
	}

	// Check if the account owner matches the claims username
	if account.AccountOwner != claims.Username {
		logger.Warn().Str("account_id", accountID.String()).Str("requested_by", claims.Username).Msg("Access denied: User not owner of account")
		WriteResponse(w, http.StatusForbidden, nil)
		return
	}

	logger.Info().Str("account_id", account.ID.String()).Msg("Successfully retrieved account")
	WriteResponse(w, http.StatusOK, *account)

}

// UpdateAccountService updates an account based on account ID from the URL path.
func (svc *BillingAccountService) UpdateAccountService(w http.ResponseWriter, r *http.Request) {

	logger := zerolog.Ctx(r.Context())

	// Parse the account ID from the URL path
	accountID, err := uuid.Parse(mux.Vars(r)["account-id"])

	if err != nil {
		logger.Warn().Err(err).Msg("Account doesn't exist")
		WriteResponse(w, http.StatusBadRequest, nil)
		return
	}

	// Decode the request payload into an Account struct
	var updatePayload ws_services.Account
	if err := json.NewDecoder(r.Body).Decode(&updatePayload); err != nil {
		logger.Warn().Err(err).Msg("Invalid update request payload")
		WriteResponse(w, http.StatusBadRequest, nil)
		return
	}

	// Call UpdateAccount to change the account fields in the database
	updatedAccount, err := svc.DB.UpdateAccount(accountID, updatePayload)
	if err != nil {
		logger.Error().Err(err).Str("account_id", accountID.String()).Msg("Database error updating account")
		WriteResponse(w, http.StatusInternalServerError, nil)
		return
	}

	logger.Info().Str("account_id", updatedAccount.ID.String()).Msg("Account updated successfully")
	WriteResponse(w, http.StatusOK, *updatedAccount)

}

// DeleteAccountService deletes an account specified by the account ID from the URL path.
func (svc *BillingAccountService) DeleteAccountService(w http.ResponseWriter, r *http.Request) {

	logger := zerolog.Ctx(r.Context())

	accountID, err := uuid.Parse(mux.Vars(r)["account-id"])
	if err != nil {
		logger.Warn().Err(err).Msg("Account doesn't exist")
		WriteResponse(w, http.StatusBadRequest, nil)
		return
	}

	// TODO: Need to send a publish message to delete all workspaces associated with the account
	err = svc.DB.DeleteAccount(accountID)

	if err != nil {
		logger.Error().Err(err).Str("account_id", accountID.String()).Msg("Database error deleting account")
		WriteResponse(w, http.StatusInternalServerError, nil)
		return
	}

	logger.Info().Str("account_id", accountID.String()).Msg("Account deleted successfully")
	WriteResponse(w, http.StatusNoContent, nil)
}

func (svc *BillingAccountService) SendEmailAPI(account *ws_services.Account) {

	// Email details
	from := "support@account-verification.dev.eodatahub.org.uk" // Must be verified in SES
	to := "jonny.langstone@telespazio.com"
	subject := "Account Verification Required"
	activationLink := "https://dev.eodatahub.org.uk/api/accounts/" + account.ID.String() + "/activate"

	bodyText := fmt.Sprintf(`
	A new billing account has been requested:

	Account Owner: %s
	Account Name: %s
	Organization Name: %s
	Billing Address: %s
	Account Opening Reason: %s
		
	Choose one of the following options:

	To approve the account, click the following link:

	%s

	To deny the account, click the following link:

	%s
	`, account.AccountOwner, account.Name, *account.OrganizationName, account.BillingAddress, *account.AccountOpeningReason, activationLink, activationLink)

	// Prepare the email input
	input := &sesv2.SendEmailInput{
		FromEmailAddress: aws.String(from),
		Destination: &types.Destination{
			ToAddresses: []string{to},
		},
		Content: &types.EmailContent{
			Simple: &types.Message{
				Subject: &types.Content{
					Data:    aws.String(subject),
					Charset: aws.String("UTF-8"),
				},
				Body: &types.Body{
					Text: &types.Content{
						Data:    aws.String(bodyText),
						Charset: aws.String("UTF-8"),
					},
				},
			},
		},
	}

	// Add a timeout context
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Send the email
	fmt.Println("Attempting to send email via SES API...")
	output, err := svc.AWSEmailClient.SendEmail(ctx, input)
	if err != nil {
		fmt.Println("Failed to send email: %v", err)
	}

	log.Printf("Email sent successfully! Message ID: %s", *output.MessageId)
}
