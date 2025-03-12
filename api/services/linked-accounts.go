package services

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/secretsmanager"
	"github.com/gorilla/mux"
	"github.com/rs/zerolog"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// Payload represents the expected JSON structure
type Payload struct {
	Name string `json:"name"`
	Key  string `json:"key"`
}

// CreateLinkedAccountService handles the creation of a linked account, encrypts the API key using OTP,
// and securely stores it in both Kubernetes (for the OTP key) and AWS Secrets Manager (for the encrypted key).
func CreateLinkedAccountService(svc *Service, w http.ResponseWriter, r *http.Request) {

	logger := zerolog.Ctx(r.Context())

	// Extract the workspace ID from the request URL path
	workspaceID := mux.Vars(r)["workspace-id"]
	namespace := "ws-" + workspaceID

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
		WriteResponse(w, http.StatusBadRequest, fmt.Sprintf("Encryption failed: %v", err))
		return
	}

	// Store OTP in Kubernetes
	k8sSecretName := "otp-" + payload.Name
	if err := storeOTPInK8sSecret(otp, k8sSecretName, namespace); err != nil {
		logger.Error().Err(err).Str("workspace_id", workspaceID).Msg(fmt.Sprintf("Failed to store OTP in Kubernetes: %v", err))
		WriteResponse(w, http.StatusInternalServerError, fmt.Sprintf("Failed to store OTP in Kubernetes: %v", err))
		return
	}

	// Store the ciphertext in AWS Secrets Manager
	awsSecretName := "ws-" + workspaceID // We want to differentiate general secrets in AWS with workspace specific secrets
	if err := storeCiphertextInAWSSecrets(ciphertext, awsSecretName, payload.Name); err != nil {
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

// storeOTPKeyInK8sSecret securely stores the OTP key in a Kubernetes secret.
// The OTP key is required to decrypt the corresponding ciphertext stored in AWS.
func storeOTPInK8sSecret(otpKey, secretName, namespace string) error {
	var config *rest.Config
	var err error

	// Check if running inside a Kubernetes pod
	if _, exists := os.LookupEnv("KUBERNETES_SERVICE_HOST"); exists {
		// Inside Kubernetes, use in-cluster config
		config, err = rest.InClusterConfig()
		if err != nil {
			return fmt.Errorf("failed to load in-cluster Kubernetes config: %v", err)
		}
	} else {
		// Running locally, use kubeconfig
		config, err = clientcmd.BuildConfigFromFlags("", clientcmd.RecommendedHomeFile)
		if err != nil {
			return fmt.Errorf("failed to load kubeconfig: %v", err)
		}
	}

	// Create Kubernetes client
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("failed to create Kubernetes client: %v", err)
	}

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
	_, err = clientset.CoreV1().Secrets(namespace).Create(context.TODO(), secret, metav1.CreateOptions{})
	if err != nil {
		// If secret already exists, update it instead
		if errors.IsAlreadyExists(err) {
			_, err = clientset.CoreV1().Secrets(namespace).Update(context.TODO(), secret, metav1.UpdateOptions{})
			if err != nil {
				return fmt.Errorf("failed to update existing Kubernetes secret: %v", err)
			}
		} else {
			return fmt.Errorf("failed to create Kubernetes secret: %v", err)
		}
	}
	return nil
}

// storeCiphertextInAWSSecrets securely stores the OTP-encrypted ciphertext in AWS Secrets Manager.
// Unlike Kubernetes, AWS Secrets Manager stores only the ciphertext, while the OTP key remains in Kubernetes.
func storeCiphertextInAWSSecrets(ciphertext string, secretName, payloadName string) error {
	sess, err := session.NewSession(&aws.Config{
		Region: aws.String("eu-west-2"),
	})
	if err != nil {
		return fmt.Errorf("failed to create AWS session: %v", err)
	}

	svc := secretsmanager.New(sess)

	// Try to get existing secret
	var existingKeys map[string]string
	getResult, err := svc.GetSecretValue(&secretsmanager.GetSecretValueInput{
		SecretId: aws.String(secretName),
	})
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok && aerr.Code() == secretsmanager.ErrCodeResourceNotFoundException {
			existingKeys = make(map[string]string)
		} else {
			return fmt.Errorf("failed to get existing secret: %v", err)
		}
	} else if getResult.SecretString != nil {
		if err := json.Unmarshal([]byte(*getResult.SecretString), &existingKeys); err != nil {
			return fmt.Errorf("failed to parse existing secret: %v", err)
		}
	} else {
		existingKeys = make(map[string]string)
	}

	// Store the OTP-encrypted ciphertext in AWS
	existingKeys[payloadName] = ciphertext

	// Marshal updated keys to JSON
	updatedSecret, err := json.Marshal(existingKeys)
	if err != nil {
		return fmt.Errorf("failed to marshal secret: %v", err)
	}

	// Create or update the secret
	input := &secretsmanager.CreateSecretInput{
		Name:         aws.String(secretName),
		SecretString: aws.String(string(updatedSecret)),
	}

	_, err = svc.CreateSecret(input)
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok && aerr.Code() == secretsmanager.ErrCodeResourceExistsException {
			_, err = svc.UpdateSecret(&secretsmanager.UpdateSecretInput{
				SecretId:     aws.String(secretName),
				SecretString: aws.String(string(updatedSecret)),
			})
			if err != nil {
				return fmt.Errorf("failed to update AWS secret: %v", err)
			}
		} else {
			return fmt.Errorf("failed to create AWS secret: %v", err)
		}
	}
	return nil
}
