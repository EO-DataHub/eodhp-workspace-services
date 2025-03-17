package services

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/EO-DataHub/eodhp-workspace-services/api/middleware"
	"github.com/EO-DataHub/eodhp-workspace-services/internal/authn"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/smithy-go"
	"github.com/gorilla/mux"
	"github.com/rs/zerolog"
	corev1 "k8s.io/api/core/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Payload represents the expected JSON structure
type Payload struct {
	Name string `json:"name"`
	Key  string `json:"key"`
}

type LinkedAccountServiceInterface interface {
	GetLinkedAccounts(w http.ResponseWriter, r *http.Request)
	DeleteLinkedAccountService(w http.ResponseWriter, r *http.Request)
	CreateLinkedAccountService(w http.ResponseWriter, r *http.Request)
	storeOTPSecret(otpKey, secretName, namespace string) error
	storeCiphertextInAWSSecrets(ciphertext, secretName, payloadName string) error
	getSecretKeysFromAWS(secretName string) ([]string, error)
	deleteOTPSecret(secretName, namespace string) error
	deleteSecretKeyFromAWS(secretName, keyToRemove string) error
}

// GetLinkedAccountsService handles the retrieval of linked accounts from AWS Secrets Manager.
// The linked account secrets are stored as key-value pairs, where the key is the provider name and the value is the encrypted API key.
func (svc *LinkedAccountService) GetLinkedAccounts(w http.ResponseWriter, r *http.Request) {

	logger := zerolog.Ctx(r.Context())

	// Extract claims from the request context to identify the user
	claims, ok := r.Context().Value(middleware.ClaimsKey).(authn.Claims)
	if !ok {
		logger.Warn().Msg("Unauthorized request: missing claims")
		WriteResponse(w, http.StatusUnauthorized, nil)
		return
	}

	// Extract the workspace ID from the request URL path
	workspaceID := mux.Vars(r)["workspace-id"]
	namespace := "ws-" + workspaceID

	// Check if the user can access the workspace
	authorized, err := isUserWorkspaceAuthorized(svc.DB, claims, workspaceID, false)
	if err != nil {
		logger.Error().Err(err).Str("workspace_id", workspaceID).Msg("Failed to authorize workspace")
		WriteResponse(w, http.StatusInternalServerError, nil)
		return
	}

	if !authorized {
		WriteResponse(w, http.StatusForbidden, nil)
		return
	}

	providers, err := svc.getSecretKeysFromAWS(namespace)

	// Return an empty array if no providers are found
	if providers == nil {
		providers = []string{}
	}

	if err != nil {
		logger.Error().Err(err).Str("workspace_id", workspaceID).Msg("Failed to get linked accounts")
		WriteResponse(w, http.StatusInternalServerError, nil)
	}

	WriteResponse(w, http.StatusOK, providers)

}

func (svc *LinkedAccountService) DeleteLinkedAccountService(w http.ResponseWriter, r *http.Request) {

	logger := zerolog.Ctx(r.Context())

	// Extract claims from the request context to identify the user
	claims, ok := r.Context().Value(middleware.ClaimsKey).(authn.Claims)
	if !ok {
		logger.Warn().Msg("Unauthorized request: missing claims")
		WriteResponse(w, http.StatusUnauthorized, nil)
		return
	}

	// Extract the workspace ID from the request URL path
	workspaceID := mux.Vars(r)["workspace-id"]
	provider := mux.Vars(r)["provider"]
	namespace := "ws-" + workspaceID

	// Check if the user is the account owner
	authorized, err := isUserWorkspaceAuthorized(svc.DB, claims, workspaceID, true)
	if err != nil {
		logger.Error().Err(err).Str("workspace_id", workspaceID).Msg("Failed to authorize workspace")
		WriteResponse(w, http.StatusInternalServerError, nil)
		return
	}

	if !authorized {
		WriteResponse(w, http.StatusForbidden, nil)
		return
	}

	// Delete the OTP secret from Kubernetes
	k8sSecretName := "otp-" + provider
	err = svc.deleteOTPSecret(k8sSecretName, namespace)
	if err != nil {
		logger.Error().Err(err).Str("workspace_id", workspaceID).Str("provider", provider).Msg("Failed to delete OTP secret")
		WriteResponse(w, http.StatusInternalServerError, nil)
		return
	}

	// Delete the encrypted key from AWS Secrets Manager
	err = svc.deleteSecretKeyFromAWS(namespace, provider)
	if err != nil {
		logger.Error().Err(err).Str("workspace_id", workspaceID).Str("provider", provider).Msg("Failed to delete encrypted key from AWS Secrets Manager")
		WriteResponse(w, http.StatusInternalServerError, nil)
		return
	}

	logger.Info().Str("workspace_id", workspaceID).Str("provider", provider).Msg("Successfully deleted linked account")

	WriteResponse(w, http.StatusNoContent, nil)

}

// CreateLinkedAccountService handles the creation of a linked account, encrypts the API key using OTP,
// and securely stores it in both Kubernetes (for the OTP key) and AWS Secrets Manager (for the encrypted key).
func (svc *LinkedAccountService) CreateLinkedAccountService(w http.ResponseWriter, r *http.Request) {

	logger := zerolog.Ctx(r.Context())

	// Extract claims from the request context to identify the user
	claims, ok := r.Context().Value(middleware.ClaimsKey).(authn.Claims)
	if !ok {
		logger.Warn().Msg("Unauthorized request: missing claims")
		WriteResponse(w, http.StatusUnauthorized, nil)
		return
	}

	// Extract the workspace ID from the request URL path
	workspaceID := mux.Vars(r)["workspace-id"]
	namespace := "ws-" + workspaceID

	// Check if the user is the account owner
	authorized, err := isUserWorkspaceAuthorized(svc.DB, claims, workspaceID, true)
	if err != nil {
		logger.Error().Err(err).Str("workspace_id", workspaceID).Msg("Failed to authorize workspace")
		WriteResponse(w, http.StatusInternalServerError, nil)
		return
	}

	if !authorized {
		WriteResponse(w, http.StatusForbidden, nil)
		return
	}

	// Read the request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		logger.Error().Err(err).Str("workspace_id", workspaceID).Msg("Failed to read request body")
		WriteResponse(w, http.StatusBadRequest, "Failed to read request body")
		return
	}
	defer r.Body.Close()

	// Parse the JSON payload
	var payload Payload
	if err := json.Unmarshal(body, &payload); err != nil {
		logger.Error().Err(err).Str("workspace_id", workspaceID).Msg("Invalid JSON payload")
		WriteResponse(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	// Validate the key field
	if payload.Key == "" {
		logger.Error().Err(err).Str("workspace_id", workspaceID).Msg("Field 'key' is required and cannot be empty")
		WriteResponse(w, http.StatusBadRequest, "Field 'key' is required and cannot be empty")
		return
	}

	// Encrypt the key field using One-Time Pad (OTP)
	otp, ciphertext, err := encryptWithOTP(payload.Key)
	if err != nil {
		logger.Error().Err(err).Str("workspace_id", workspaceID).Msg(fmt.Sprintf("Encryption failed: %v", err))
		WriteResponse(w, http.StatusInternalServerError, fmt.Sprintf("Encryption failed: %v", err))
		return
	}

	// Store OTP in Kubernetes
	k8sSecretName := "otp-" + payload.Name
	if err := svc.storeOTPSecret(otp, k8sSecretName, namespace); err != nil {
		logger.Error().Err(err).Str("workspace_id", workspaceID).Msg(fmt.Sprintf("Failed to store OTP in Kubernetes: %v", err))
		WriteResponse(w, http.StatusInternalServerError, fmt.Sprintf("Failed to store OTP in Kubernetes: %v", err))
		return
	}
	// Store the ciphertext in AWS Secrets Manager
	awsSecretName := "ws-" + workspaceID // We want to differentiate general secrets in AWS with workspace specific secrets
	if err := svc.storeCiphertextInAWSSecrets(ciphertext, awsSecretName, payload.Name); err != nil {
		logger.Error().Err(err).Str("workspace_id", workspaceID).Msg(fmt.Sprintf("Failed to store encrypted key in AWS: %v", err))
		WriteResponse(w, http.StatusInternalServerError, fmt.Sprintf("Failed to store encrypted key in AWS: %v", err))
		return
	}
	WriteResponse(w, http.StatusCreated, nil)

}

// encryptWithOTP uses a One-Time Pad (OTP) to securely encrypt data by XORing it with a random key that is the same length as the plaintext and used only once to ensure perfect secrecy.
func encryptWithOTP(plaintext string) (string, string, error) {

	plaintextBytes := []byte(plaintext)

	// Generate a truly random OTP key of the same length as the plaintext
	otpKey := make([]byte, len(plaintextBytes))
	if _, err := rand.Read(otpKey); err != nil {
		return "", "", fmt.Errorf("failed to generate OTP key: %v", err)
	}

	// XOR the plaintext with the OTP key to generate the ciphertext
	ciphertextBytes := make([]byte, len(plaintextBytes))
	for i := range plaintextBytes {
		ciphertextBytes[i] = plaintextBytes[i] ^ otpKey[i] // XOR operation
	}

	// Encode both OTP key and ciphertext in Base64 for storage
	encodedOTPKey := base64.StdEncoding.EncodeToString(otpKey)
	encodedCiphertext := base64.StdEncoding.EncodeToString(ciphertextBytes)

	return encodedOTPKey, encodedCiphertext, nil
}

// storeOTPSecret securely stores the OTP key in a Kubernetes secret.
// The OTP key is required to decrypt the corresponding ciphertext stored in AWS.
func (svc *LinkedAccountService) storeOTPSecret(otpKey, secretName, namespace string) error {

	// Decode OTP if it's already Base64-encoded
	decodedOTPKey, err := base64.StdEncoding.DecodeString(otpKey)
	if err != nil {
		return fmt.Errorf("failed to decode OTP before storing: %v", err)
	}

	// Create Kubernetes secret object
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: secretName,
		},
		Data: map[string][]byte{
			"otp": decodedOTPKey,
		},
	}

	// Try creating the secret
	_, err = svc.K8sClient.CoreV1().Secrets(namespace).Create(context.TODO(), secret, metav1.CreateOptions{})
	if err != nil {
		// If secret already exists, update it instead
		if k8sErrors.IsAlreadyExists(err) {
			_, err = svc.K8sClient.CoreV1().Secrets(namespace).Update(context.TODO(), secret, metav1.UpdateOptions{})
			if err != nil {
				return fmt.Errorf("failed to update existing Kubernetes secret: %v", err)
			}
		} else {
			return fmt.Errorf("failed to create Kubernetes secret: %v", err)
		}
	}
	return nil
}

// storeCiphertextInAWSSecrets stores a key-value pair in AWS Secrets Manager
func (svc *LinkedAccountService) storeCiphertextInAWSSecrets(ciphertext, secretName, payloadName string) error {
	ctx := context.Background()
	var existingKeys map[string]string

	// Try to get the existing secret
	getResult, err := svc.SecretsManager.GetSecretValue(ctx, &secretsmanager.GetSecretValueInput{
		SecretId: aws.String(secretName),
	})
	if err != nil {
		var opErr *smithy.OperationError
		if errors.As(err, &opErr) {
			if strings.Contains(opErr.Unwrap().Error(), "ResourceNotFoundException") {
				existingKeys = make(map[string]string) // Initialize new secret
			} else {
				return fmt.Errorf("failed to retrieve secret: %v", err)
			}
		}

	} else if getResult.SecretString != nil {
		// Parse existing secret JSON
		existingKeys = make(map[string]string)
		if err := json.Unmarshal([]byte(*getResult.SecretString), &existingKeys); err != nil {
			return fmt.Errorf("failed to parse existing secret JSON: %v", err)
		}
	}

	// Add/Update key-value pair
	existingKeys[payloadName] = ciphertext
	updatedSecret, err := json.Marshal(existingKeys)
	if err != nil {
		return fmt.Errorf("failed to marshal secret JSON: %v", err)
	}

	// Store the secret (Create or Update)
	_, err = svc.SecretsManager.PutSecretValue(ctx, &secretsmanager.PutSecretValueInput{
		SecretId:     aws.String(secretName),
		SecretString: aws.String(string(updatedSecret)),
	})
	if err != nil {

		var opErr *smithy.OperationError
		if errors.As(err, &opErr) {
			if strings.Contains(opErr.Unwrap().Error(), "ResourceNotFoundException") {

				// Secret doesn't exist, create a new one
				_, err = svc.SecretsManager.CreateSecret(ctx, &secretsmanager.CreateSecretInput{
					Name:         aws.String(secretName),
					SecretString: aws.String(string(updatedSecret)),
				})
				if err != nil {
					return fmt.Errorf("failed to create secret: %v", err)
				}
			} else {
				return fmt.Errorf("failed to store/update secret: %v", err)
			}
		}
		return nil
	}
	return nil
}

func (svc *LinkedAccountService) getSecretKeysFromAWS(secretName string) ([]string, error) {

	// Fetch existing secret
	getResult, err := svc.SecretsManager.GetSecretValue(context.Background(), &secretsmanager.GetSecretValueInput{
		SecretId: aws.String(secretName),
	})

	if err != nil {
		var opErr *smithy.OperationError
		if errors.As(err, &opErr) {
			if strings.Contains(opErr.Unwrap().Error(), "ResourceNotFoundException") {
				return []string{}, nil
			} else {
				return nil, fmt.Errorf("failed to get secret: %v", err)
			}
		}
	}

	if getResult.SecretString == nil {
		return nil, fmt.Errorf("secret is empty or missing")
	}

	// Parse the secret JSON
	var existingKeys map[string]interface{}
	if err := json.Unmarshal([]byte(*getResult.SecretString), &existingKeys); err != nil {
		return nil, fmt.Errorf("failed to parse secret: %v", err)
	}

	// Extract the keys from the map
	keys := make([]string, 0, len(existingKeys))
	for key := range existingKeys {
		keys = append(keys, key)
	}

	return keys, nil
}

func (svc *LinkedAccountService) deleteOTPSecret(secretName, namespace string) error {

	// Attempt to delete the secret
	err := svc.K8sClient.CoreV1().Secrets(namespace).Delete(context.TODO(), secretName, metav1.DeleteOptions{})
	if err != nil {
		return fmt.Errorf("failed to delete Kubernetes secret: %v", err)
	}

	return nil
}

// removeKeyFromAWSSecret removes a specific key from an existing AWS secret.
func (svc *LinkedAccountService) deleteSecretKeyFromAWS(secretName, keyToRemove string) error {

	// Fetch the existing secret
	getResult, err := svc.SecretsManager.GetSecretValue(context.Background(), &secretsmanager.GetSecretValueInput{
		SecretId: aws.String(secretName),
	})
	if err != nil {
		return fmt.Errorf("failed to get secret: %v", err)
	}

	if getResult.SecretString == nil {
		return fmt.Errorf("secret is empty or missing")
	}

	// Parse the secret JSON
	var secretData map[string]string
	if err := json.Unmarshal([]byte(*getResult.SecretString), &secretData); err != nil {
		return fmt.Errorf("failed to parse secret: %v", err)
	}

	// Remove the specified key
	delete(secretData, keyToRemove)

	// Marshal the updated secret back to JSON
	updatedSecret, err := json.Marshal(secretData)
	if err != nil {
		return fmt.Errorf("failed to marshal updated secret: %v", err)
	}

	// Update the secret in AWS
	_, err = svc.SecretsManager.UpdateSecret(context.Background(), &secretsmanager.UpdateSecretInput{
		SecretId:     aws.String(secretName),
		SecretString: aws.String(string(updatedSecret)),
	})
	if err != nil {
		return fmt.Errorf("failed to update AWS secret: %v", err)
	}

	return nil
}
