package services

import (
	"testing"

	"github.com/aws/aws-sdk-go/service/secretsmanager"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	corev1 "k8s.io/api/core/v1"
)

func TestEncryptWithOTP(t *testing.T) {
	plaintext := "my-secret"
	otp, ciphertext, err := encryptWithOTP(plaintext)

	assert.NoError(t, err)
	assert.NotEqual(t, plaintext, ciphertext, "Encryption should change the plaintext")
	assert.Equal(t, len(otp), len(ciphertext), "OTP and ciphertext should have the same length")
}

// Mock Kubernetes client
type MockKubernetesClient struct {
	mock.Mock
}

func (m *MockKubernetesClient) CreateSecret(namespace string, secret *corev1.Secret) (*corev1.Secret, error) {
	args := m.Called(namespace, secret)
	return args.Get(0).(*corev1.Secret), args.Error(1)
}

func (m *MockKubernetesClient) UpdateSecret(namespace string, secret *corev1.Secret) (*corev1.Secret, error) {
	args := m.Called(namespace, secret)
	return args.Get(0).(*corev1.Secret), args.Error(1)
}

type MockSecretsManager struct {
	mock.Mock
}

func (m *MockSecretsManager) CreateSecret(input *secretsmanager.CreateSecretInput) (*secretsmanager.CreateSecretOutput, error) {
	args := m.Called(input)
	return args.Get(0).(*secretsmanager.CreateSecretOutput), args.Error(1)
}

func (m *MockSecretsManager) UpdateSecret(input *secretsmanager.UpdateSecretInput) (*secretsmanager.UpdateSecretOutput, error) {
	args := m.Called(input)
	return args.Get(0).(*secretsmanager.UpdateSecretOutput), args.Error(1)
}
