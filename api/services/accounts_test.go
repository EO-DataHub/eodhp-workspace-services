package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/EO-DataHub/eodhp-workspace-services/api/middleware"
	"github.com/EO-DataHub/eodhp-workspace-services/internal/appconfig"
	"github.com/EO-DataHub/eodhp-workspace-services/internal/authn"
	"github.com/EO-DataHub/eodhp-workspace-services/models"
	"github.com/aws/aws-sdk-go-v2/service/sesv2"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestCreateAccountApprovalToken(t *testing.T) {

	mockDB := new(MockWorkspaceDB)

	mockDB.On("CreateAccountApprovalToken", mock.AnythingOfType("uuid.UUID")).Return("some-token", nil)

	token, err := mockDB.CreateAccountApprovalToken(uuid.New())

	mockDB.AssertExpectations(t)

	// Check if the return value is correct
	if token != "some-token" || err != nil {
		t.Errorf("Expected token 'some-token' but got '%s' and error '%v'", token, err)
	}
}

func TestCreateAccountService(t *testing.T) {

	// Create a mock database, email client, and config
	mockDB := new(MockWorkspaceDB)
	mockAWSEmailClient := new(MockAWSEmailClient)
	mockConfig := &appconfig.Config{
		Accounts: appconfig.AccountsConfig{
			ServiceAccountEmail: "service@example.com",
			HelpdeskEmail:       "helpdesk@example.com",
		},
	}

	// Create the service with the mock DB, email client, and config
	svc := BillingAccountService{DB: mockDB, AWSEmailClient: mockAWSEmailClient, Config: mockConfig}

	// Define the account to be created
	testAccount := &models.Account{
		ID:                   uuid.New(),
		Name:                 "Test Account",
		AccountOwner:         "testuser",
		OrganizationName:     aws.String("Telespazio UK"),
		BillingAddress:       "123 Test St, London, UK",
		AccountOpeningReason: aws.String("Testing"),
	}

	// Mock the JWT token and claims (simulate the authenticated user)
	mockClaims := authn.Claims{
		Username: "testuser",
	}

	// Set up the mock for CreateAccount method
	mockDB.On("CreateAccount", mock.Anything).Return(testAccount, nil)

	// Set up the mock for CreateAccountApprovalToken method
	mockDB.On("CreateAccountApprovalToken", mock.AnythingOfType("uuid.UUID")).Return("some-token", nil)

	// Mock the email client to return a successful response
	mockAWSEmailClient.On("SendEmail", mock.Anything, mock.Anything, mock.Anything).
		Return(&sesv2.SendEmailOutput{}, nil)

	// Create the request body and attach it to the request
	requestBody, _ := json.Marshal(testAccount)
	r := httptest.NewRequest(http.MethodPost, "/api/accounts", bytes.NewReader(requestBody))

	// Create a new context with the mocked claims and attach it to the request
	ctx := context.WithValue(r.Context(), middleware.ClaimsKey, mockClaims)
	r = r.WithContext(ctx)

	// Create the recorder to capture the response
	w := httptest.NewRecorder()

	// Call the service method
	svc.CreateAccountService(w, r)

	// Get the response and assert results
	res := w.Result()
	defer res.Body.Close()

	assert.Equal(t, http.StatusCreated, res.StatusCode)

	body, _ := io.ReadAll(res.Body)
	var responseAccount models.Account
	err := json.Unmarshal(body, &responseAccount)
	assert.NoError(t, err, "Response should be valid JSON")
	assert.Equal(t, testAccount.Name, responseAccount.Name, "Account name should match")
	assert.Equal(t, *testAccount.OrganizationName, *responseAccount.OrganizationName, "Organization name should match")

	mockDB.AssertExpectations(t)
	mockAWSEmailClient.AssertExpectations(t)

	mockAWSEmailClient.AssertCalled(t, "SendEmail", mock.Anything, mock.MatchedBy(func(input *sesv2.SendEmailInput) bool {
		return input.FromEmailAddress != nil && *input.FromEmailAddress == "service@example.com"
	}), mock.Anything)

	mockDB.AssertExpectations(t)
}

func TestGetAccountsService(t *testing.T) {

	mockDB := new(MockWorkspaceDB)
	mockAccounts := []models.Account{
		{ID: uuid.New(), Name: "Test Account 1", AccountOwner: "testuser"},
		{ID: uuid.New(), Name: "Test Account 2", AccountOwner: "testuser"},
	}

	mockDB.On("GetAccounts", "testuser").Return(mockAccounts, nil)

	mockClaims := authn.Claims{
		Username: "testuser",
	}

	r := httptest.NewRequest(http.MethodGet, "/api/accounts", nil)

	ctx := context.WithValue(r.Context(), middleware.ClaimsKey, mockClaims)
	r = r.WithContext(ctx)

	svc := BillingAccountService{DB: mockDB}

	w := httptest.NewRecorder()
	svc.GetAccountsService(w, r)

	res := w.Result()
	defer res.Body.Close()

	assert.Equal(t, http.StatusOK, res.StatusCode, "Expected status 200 OK")

	assert.Equal(t, "application/json", res.Header.Get("Content-Type"), "Expected JSON response")

	body, _ := io.ReadAll(res.Body)
	var responseAccounts []models.Account
	err := json.Unmarshal(body, &responseAccounts)

	assert.NoError(t, err, "Response should be valid JSON")
	assert.Len(t, responseAccounts, len(mockAccounts), "Expected number of accounts to match")

	for i, acc := range mockAccounts {
		assert.Equal(t, acc.ID, responseAccounts[i].ID, "Account ID should match")
		assert.Equal(t, acc.Name, responseAccounts[i].Name, "Account name should match")
		assert.Equal(t, acc.AccountOwner, responseAccounts[i].AccountOwner, "Account owner should match")
	}

	mockDB.AssertExpectations(t)
	mockDB.AssertCalled(t, "GetAccounts", "testuser")
}

func TestDeleteAccountService(t *testing.T) {

	mockDB := new(MockWorkspaceDB)
	accountID := uuid.New()

	mockClaims := authn.Claims{
		Username: "testuser",
	}

	mockDB.On("DeleteAccount", accountID).Return(nil).Once()

	req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/accounts/%s", accountID), nil)
	req = mux.SetURLVars(req, map[string]string{"account-id": accountID.String()})

	ctx := context.WithValue(req.Context(), middleware.ClaimsKey, mockClaims)
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()

	svc := BillingAccountService{DB: mockDB}
	svc.DeleteAccountService(w, req)

	res := w.Result()
	defer res.Body.Close()

	assert.Equal(t, http.StatusNoContent, res.StatusCode, "Expected HTTP status 204 No Content")

	mockDB.AssertExpectations(t)
	mockDB.AssertCalled(t, "DeleteAccount", accountID)

}

func TestGetAccountService(t *testing.T) {

	mockDB := new(MockWorkspaceDB)
	accountID := uuid.New()

	mockAccount := &models.Account{
		ID:           accountID,
		Name:         "Test Account",
		AccountOwner: "testuser",
	}

	mockClaims := authn.Claims{
		Username: "testuser",
	}

	svc := BillingAccountService{DB: mockDB}

	mockDB.On("GetAccount", accountID).Return(mockAccount, nil).Once()

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/accounts/%s", accountID), nil)
	req = mux.SetURLVars(req, map[string]string{"account-id": accountID.String()})

	ctx := context.WithValue(req.Context(), middleware.ClaimsKey, mockClaims)
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()

	svc.GetAccountService(w, req)

	res := w.Result()
	defer res.Body.Close()

	assert.Equal(t, http.StatusOK, res.StatusCode, "Expected HTTP status 200 OK")

	mockDB.AssertCalled(t, "GetAccount", accountID)

}
