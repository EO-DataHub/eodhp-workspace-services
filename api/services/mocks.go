package services

import (
	"context"
	"database/sql"

	ws_manager "github.com/EO-DataHub/eodhp-workspace-manager/models"
	"github.com/EO-DataHub/eodhp-workspace-services/models"
	ws_services "github.com/EO-DataHub/eodhp-workspace-services/models"
	"github.com/aws/aws-sdk-go-v2/service/sesv2"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
)

type MockAWSEmailClient struct {
	mock.Mock
}

type MockWorkspaceDB struct {
	mock.Mock
}

type MockKeycloakClient struct {
	mock.Mock
}

type MockEventPublisher struct {
	mock.Mock
}

func (m *MockAWSEmailClient) SendEmail(ctx context.Context, input *sesv2.SendEmailInput, opts ...func(*sesv2.Options)) (*sesv2.SendEmailOutput, error) {
	args := m.Called(ctx, input, opts)
	return args.Get(0).(*sesv2.SendEmailOutput), args.Error(1)
}

func (m *MockWorkspaceDB) CreateAccount(account *ws_services.Account) (*ws_services.Account, error) {
	args := m.Called(account)
	return args.Get(0).(*ws_services.Account), args.Error(1)
}

func (m *MockWorkspaceDB) GetAccounts(username string) ([]models.Account, error) {
	args := m.Called(username)
	return args.Get(0).([]models.Account), args.Error(1)
}

func (m *MockWorkspaceDB) GetAccount(accountID uuid.UUID) (*ws_services.Account, error) {
	args := m.Called(accountID)
	return args.Get(0).(*ws_services.Account), args.Error(1)
}

func (m *MockWorkspaceDB) UpdateAccount(accountID uuid.UUID, account ws_services.Account) (*ws_services.Account, error) {
	args := m.Called(accountID, account)
	return args.Get(0).(*ws_services.Account), args.Error(1)
}

func (m *MockWorkspaceDB) DeleteAccount(accountID uuid.UUID) error {
	args := m.Called(accountID)
	return args.Error(0)
}

func (m *MockWorkspaceDB) IsUserAccountOwner(username, workspaceID string) (bool, error) {
	args := m.Called(username, workspaceID)
	return args.Bool(0), args.Error(1)
}

func (m *MockWorkspaceDB) CreateAccountApprovalToken(accountID uuid.UUID) (string, error) {
	args := m.Called(accountID)
	return args.String(0), args.Error(1)
}

func (m *MockWorkspaceDB) ValidateApprovalToken(token string) (string, error) {
	args := m.Called(token)
	return args.String(0), args.Error(1)
}

func (m *MockWorkspaceDB) UpdateAccountStatus(token, accountID, status string) error {
	args := m.Called(token, accountID, status)
	return args.Error(0)
}

func (m *MockWorkspaceDB) Close() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockWorkspaceDB) CheckAccountIsVerified(accountID uuid.UUID) (bool, error) {
	args := m.Called(accountID)
	return args.Bool(0), args.Error(1)
}

func (m *MockWorkspaceDB) GetWorkspace(workspaceName string) (*ws_manager.WorkspaceSettings, error) {
	args := m.Called(workspaceName)
	return args.Get(0).(*ws_manager.WorkspaceSettings), args.Error(1)
}

func (m *MockWorkspaceDB) GetUserWorkspaces(memberGroups []string) ([]ws_manager.WorkspaceSettings, error) {
	args := m.Called(memberGroups)
	return args.Get(0).([]ws_manager.WorkspaceSettings), args.Error(1)
}

func (m *MockWorkspaceDB) GetOwnedWorkspaces(username string) ([]ws_manager.WorkspaceSettings, error) {
	args := m.Called(username)
	return args.Get(0).([]ws_manager.WorkspaceSettings), args.Error(1)
}

func (m *MockWorkspaceDB) CheckWorkspaceExists(name string) (bool, error) {
	args := m.Called(name)
	return args.Bool(0), args.Error(1)
}

func (m *MockWorkspaceDB) UpdateWorkspaceStatus(status ws_manager.WorkspaceStatus) error {
	args := m.Called(status)
	return args.Error(0)
}

func (m *MockWorkspaceDB) DisableWorkspace(workspaceName string) error {
	args := m.Called(workspaceName)
	return args.Error(0)
}

func (m *MockWorkspaceDB) CreateWorkspace(req *ws_manager.WorkspaceSettings) (*sql.Tx, error) {
	args := m.Called(req)
	return args.Get(0).(*sql.Tx), args.Error(1)
}

func (m *MockWorkspaceDB) CommitTransaction(tx *sql.Tx) error {
	args := m.Called(tx)
	return args.Error(0)
}

// CreateUser mock
func (m *MockKeycloakClient) CreateUser(username, email, password string) (string, error) {
	args := m.Called(username, email, password)
	return args.String(0), args.Error(1)
}

// DeleteUser mock
func (m *MockKeycloakClient) DeleteUser(userID string) error {
	args := m.Called(userID)
	return args.Error(0)
}

// CreateGroup mock
func (m *MockKeycloakClient) CreateGroup(groupName string) (int, error) {
	args := m.Called(groupName)
	return args.Get(0).(int), args.Error(1)
}

// DeleteGroup mock (This was missing)
func (m *MockKeycloakClient) DeleteGroup(groupID string) (int, error) {
	args := m.Called(groupID)
	return args.Get(0).(int), args.Error(0)
}

// AddUserToGroup mock
func (m *MockKeycloakClient) AddUserToGroup(userID, groupID string) error {
	args := m.Called(userID, groupID)
	return args.Error(0)
}

// RemoveUserFromGroup mock
func (m *MockKeycloakClient) RemoveUserFromGroup(userID, groupID string) error {
	args := m.Called(userID, groupID)
	return args.Error(0)
}

func (m *MockKeycloakClient) GetToken() error {
	return nil // No-op for mock
}

// AddMemberToGroup mock (Add this method)
func (m *MockKeycloakClient) AddMemberToGroup(userID, groupID string) error {
	args := m.Called(userID, groupID)
	return args.Error(0)
}

func (m *MockKeycloakClient) ExchangeToken(accessToken, scope string) (*TokenResponse, error) {
	args := m.Called(accessToken, scope)
	return args.Get(0).(*TokenResponse), args.Error(1)
}

func (m *MockKeycloakClient) GetGroup(groupName string) (*models.Group, error) {
	args := m.Called(groupName)
	return args.Get(0).(*models.Group), args.Error(1)
}

func (m *MockKeycloakClient) GetGroupMembers(groupID string) ([]models.User, error) {
	args := m.Called(groupID)
	return args.Get(0).([]models.User), args.Error(1)
}

func (m *MockKeycloakClient) GetGroupMember(groupID, userID string) (*models.User, error) {
	args := m.Called(groupID)
	return args.Get(0).(*models.User), args.Error(1)
}

func (m *MockKeycloakClient) GetUser(username string) (*models.User, error) {
	args := m.Called(username)
	return args.Get(0).(*models.User), args.Error(1)
}

func (m *MockKeycloakClient) GetUserGroups(userID string) ([]string, error) {
	args := m.Called(userID)
	return args.Get(0).([]string), args.Error(1)
}
func (m *MockKeycloakClient) RemoveMemberFromGroup(userID, groupID string) error {
	args := m.Called(userID, groupID)
	return args.Error(1)
}

// Mock the Publish method
func (m *MockEventPublisher) Publish(event ws_manager.WorkspaceSettings) error {
	args := m.Called(event)
	return args.Error(0)
}

// Mock the Close method
func (m *MockEventPublisher) Close() {
	m.Called()
}
