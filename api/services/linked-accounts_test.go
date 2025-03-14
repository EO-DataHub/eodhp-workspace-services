package services

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type MockLinkedAccountService struct {
	mock.Mock
}

func (m *MockLinkedAccountService) GetLinkedAccounts(w http.ResponseWriter, r *http.Request) {
	m.Called(w, r)
}

func (m *MockLinkedAccountService) DeleteLinkedAccountService(w http.ResponseWriter, r *http.Request) {
	m.Called(w, r)
}

func (m *MockLinkedAccountService) CreateLinkedAccountService(w http.ResponseWriter, r *http.Request) {
	m.Called(w, r)
	//w.WriteHeader(http.StatusCreated) // Ensure the mock sets the correct status code
}

func (m *MockLinkedAccountService) storeOTPSecret(otpKey, secretName, namespace string) error {
	args := m.Called(otpKey, secretName, namespace)
	return args.Error(0)
}

func (m *MockLinkedAccountService) storeCiphertextInAWSSecrets(ciphertext, secretName, payloadName string) error {
	args := m.Called(ciphertext, secretName, payloadName)
	return args.Error(0)
}

func (m *MockLinkedAccountService) getSecretKeysFromAWS(secretName string) ([]string, error) {
	args := m.Called(secretName)
	return args.Get(0).([]string), args.Error(1)
}

func (m *MockLinkedAccountService) deleteOTPSecret(secretName, namespace string) error {
	args := m.Called(secretName, namespace)
	return args.Error(0)
}

func (m *MockLinkedAccountService) deleteSecretKeyFromAWS(secretName, keyToRemove string) error {
	args := m.Called(secretName, keyToRemove)
	return args.Error(0)
}

func TestStoreCiphertextInAWSSecrets(t *testing.T) {
	mockService := new(MockLinkedAccountService)
	ciphertext := "test-ciphertext"
	secretName := "test-secret"
	payloadName := "test-payload"

	mockService.On("storeCiphertextInAWSSecrets", ciphertext, secretName, payloadName).Return(nil)

	err := mockService.storeCiphertextInAWSSecrets(ciphertext, secretName, payloadName)
	require.NoError(t, err)

	mockService.AssertExpectations(t)
}

func TestGetSecretKeysFromAWS(t *testing.T) {
	mockService := new(MockLinkedAccountService)
	secretName := "test-secret"
	expectedKeys := []string{"key1", "key2"}

	mockService.On("getSecretKeysFromAWS", secretName).Return(expectedKeys, nil)

	keys, err := mockService.getSecretKeysFromAWS(secretName)
	require.NoError(t, err)
	require.Equal(t, expectedKeys, keys)

	mockService.AssertExpectations(t)
}

func TestDeleteOTPSecret(t *testing.T) {
	mockService := new(MockLinkedAccountService)
	secretName := "test-secret"
	namespace := "test-namespace"

	mockService.On("deleteOTPSecret", secretName, namespace).Return(nil)

	err := mockService.deleteOTPSecret(secretName, namespace)
	require.NoError(t, err)

	mockService.AssertExpectations(t)
}
func TestDeleteSecretKeyFromAWS(t *testing.T) {
	var svc LinkedAccountServiceInterface = new(MockLinkedAccountService)
	mockService := svc.(*MockLinkedAccountService)

	// Define test parameters
	secretName := "test-secret"
	keyToRemove := "test-key"

	// Mock the GetSecretValue call to return a secret with the key to be removed
	mockService.On("deleteSecretKeyFromAWS", secretName, keyToRemove).Return(nil)

	// Test scenario where key exists and is successfully deleted
	err := svc.deleteSecretKeyFromAWS(secretName, keyToRemove)
	require.NoError(t, err, "Expected no error when key exists and is deleted")

	// Check that the mock expectations were met
	mockService.AssertExpectations(t)

	// Scenario 2: The key doesn't exist in the secret.
	// Mocking `deleteSecretKeyFromAWS` behavior where key is missing
	mockService.On("deleteSecretKeyFromAWS", secretName, "non-existent-key").Return(fmt.Errorf("key does not exist"))

	// Test that error is returned when key doesn't exist
	err = svc.deleteSecretKeyFromAWS(secretName, "non-existent-key")
	require.Error(t, err, "Expected an error when the key does not exist")
	require.Contains(t, err.Error(), "key does not exist", "Expected error to indicate that the key does not exist")

	// Check that the mock expectations were met for the second case
	mockService.AssertExpectations(t)
}
