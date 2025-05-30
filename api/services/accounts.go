package services

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"net/http"

	"github.com/EO-DataHub/eodhp-workspace-services/api/middleware"
	"github.com/EO-DataHub/eodhp-workspace-services/db"
	"github.com/EO-DataHub/eodhp-workspace-services/internal/appconfig"
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

var (
	AccountStatusApproved = "Approved"
	AccountStatusDenied   = "Denied"
	AccountStatusPending  = "Pending"
)

type EmailClient interface {
	SendEmail(ctx context.Context, input *sesv2.SendEmailInput, optFns ...func(*sesv2.Options)) (*sesv2.SendEmailOutput, error)
}

type BillingAccountService struct {
	Config         *appconfig.Config
	DB             db.WorkspaceDBInterface
	AWSEmailClient EmailClient
	KC             KeycloakClientInterface
}

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

	token, err := svc.DB.CreateAccountApprovalToken(account.ID)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to create account approval token")
		WriteResponse(w, http.StatusInternalServerError, nil)
		return
	}

	err = svc.SendAccountRequestEmail(account, token)

	if err != nil {
		logger.Error().Err(err).Msg("Failed to send account request email")
		WriteResponse(w, http.StatusInternalServerError, nil)
		return
	}

	logger.Info().Str("account_id", account.ID.String()).Msg(fmt.Sprintf("Account %s has been created by %s and is awaiting approval", account.Name, account.AccountOwner))

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

// DeleteAccountService deletes an account specified by the account ID from the URL path.
func (svc *BillingAccountService) AccountApprovalService(w http.ResponseWriter, r *http.Request, accountStatusRequest string) {

	logger := zerolog.Ctx(r.Context())

	vars := mux.Vars(r)
	token := vars["token"]

	if token == "" {
		logger.Warn().Msg("Token is required")
		http.Error(w, "Token is required", http.StatusBadRequest)
		return
	}

	// Validate token
	accountID, err := svc.DB.ValidateApprovalToken(token)
	if err != nil {
		logger.Error().Err(err).Msg("Invalid or expired token")
		http.Error(w, "Invalid or expired token", http.StatusBadRequest)
		return
	}

	// Convert accountID to UUID
	parsedID, err := uuid.Parse(accountID)
	if err != nil {
		logger.Error().Err(err).Msg("Invalid account id")
		http.Error(w, "Invalid account id", http.StatusBadRequest)
		return
	}

	// Retrieve the account from the database
	account, err := svc.DB.GetAccount(parsedID)
	if err != nil {
		logger.Error().Err(err).Msg("Database error retrieving account")
		WriteResponse(w, http.StatusInternalServerError, nil)
		return
	}

	// Find the email address of the account owner
	user, err := svc.KC.GetUser(account.AccountOwner)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to get user from Keycloak")
		WriteResponse(w, http.StatusInternalServerError, nil)
		return
	}

	// Update the account status in the database
	if err := svc.DB.UpdateAccountStatus(token, accountID, accountStatusRequest); err != nil {
		logger.Error().Err(err).Msg("Failed to update account status")
		WriteResponse(w, http.StatusInternalServerError, nil)
		return
	}

	// Send confirmation email to the account owner
	if accountStatusRequest == AccountStatusApproved {
		err = svc.SendAccountApprovalEmail(account, user.Email)
		if err != nil {
			logger.Error().Err(err).Msg("Failed to send account confirmation email")
			WriteResponse(w, http.StatusInternalServerError, nil)
			return
		}
		logger.Info().Str("account_id", account.ID.String()).Msg("Account confirmation email sent successfully")
	} else if accountStatusRequest == AccountStatusDenied {
		err = svc.SendAccountDenialEmail(account, user.Email)
		if err != nil {
			logger.Error().Err(err).Msg("Failed to send account denial email")
			WriteResponse(w, http.StatusInternalServerError, nil)
			return
		}
		logger.Info().Str("account_id", account.ID.String()).Msg("Account denial email sent successfully")
	}

	WriteResponse(w, http.StatusOK, fmt.Sprintf("Account has been %s", accountStatusRequest))
}

// SendAccountRequestEmail sends an email to the helpdesk with the account request details.
func (svc *BillingAccountService) SendAccountRequestEmail(account *ws_services.Account, token string) error {
	subject := fmt.Sprintf("EO DataHub Account Request - %s", account.AccountOwner)
	approvalLink := fmt.Sprintf("https://%s/api/accounts/admin/approve/%s", svc.Config.Host, token)
	denialLink := fmt.Sprintf("https://%s/api/accounts/admin/deny/%s", svc.Config.Host, token)

	body := fmt.Sprintf(`
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

	Make sure you are authenticated to the EO DataHub and logged in before clicking a link.
	`, account.AccountOwner, account.Name, *account.OrganizationName, account.BillingAddress, *account.AccountOpeningReason, approvalLink, denialLink)

	return svc.sendEmail(svc.Config.Accounts.ServiceAccountEmail, svc.Config.Accounts.HelpdeskEmail, subject, body)
}

// SendAccountApprovalEmail sends an email to the account owner with the account approval details.
func (svc *BillingAccountService) SendAccountApprovalEmail(account *ws_services.Account, recipient string) error {
	subject := fmt.Sprintf("EO DataHub Billing Account Confirmation - %s", account.Name)

	body := fmt.Sprintf(`
	Dear %s,

	We are pleased to inform you that your billing account has been successfully approved. 
	Thank you for your patience throughout the approval process.

	Below are the details of your approved account:

	Account Owner: %s
	Account Name: %s
	Organization Name: %s
	Billing Address: %s
	Account Opening Reason: %s

	You can now begin setting up workspaces through the EO DataHub platform. To get started, visit https://%s/workspaces/.

	A workspace is essential to fully utilize the EO DataHub. It provides a secure, hosted environment for storing workflows, 
	datasets, and results. With a workspace, you can analyze data, process datasets, place commercial orders, and generate value-added outputs 
	directly on the Hub.

	For guidance on how to create and manage your workspaces, please refer to our documentation at https://%s/docs/account-setup/workspaces/.

	If you have any questions or require assistance, please dont hesitate to contact our support team at enquiries@eodatahub.org.uk.

	Regards,
	EO DataHub Team
	`, account.AccountOwner, account.AccountOwner, account.Name, *account.OrganizationName, account.BillingAddress,
		*account.AccountOpeningReason, svc.Config.Host, svc.Config.Host)

	return svc.sendEmail(svc.Config.Accounts.ServiceAccountEmail, recipient, subject, body)
}

// SendAccountDenialEmail sends an email to the account owner with the account denial details.
func (svc *BillingAccountService) SendAccountDenialEmail(account *ws_services.Account, recipient string) error {
	subject := fmt.Sprintf("EO DataHub Billing Account Denial - %s", account.Name)

	body := fmt.Sprintf(`
	Dear %s,

	Thank you for your interest in EO DataHub. After reviewing your account request, 
	we regret to inform you that your billing account application has not been approved at this time.

	Below is a summary of the submitted account details:

	Account Owner: %s
	Account Name: %s
	Organization Name: %s
	Billing Address: %s
	Account Opening Reason: %s

	We are sorry for the inconvenience. Please do not hesitate to contact us if you have any questions.

	Regards,
	EO DataHub Team
	`, account.AccountOwner, account.AccountOwner, account.Name, *account.OrganizationName, account.BillingAddress, *account.AccountOpeningReason)

	return svc.sendEmail(svc.Config.Accounts.ServiceAccountEmail, recipient, subject, body)
}

// sendEmail is a shared helper to construct and send an SES email
func (svc *BillingAccountService) sendEmail(from, to, subject, body string) error {
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
						Data:    aws.String(body),
						Charset: aws.String("UTF-8"),
					},
				},
			},
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := svc.AWSEmailClient.SendEmail(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to send email: %v", err)
	}
	return nil
}
