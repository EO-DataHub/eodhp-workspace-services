package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/EO-DataHub/eodhp-workspace-services/api/middleware"
	"github.com/EO-DataHub/eodhp-workspace-services/internal/appconfig"
	"github.com/EO-DataHub/eodhp-workspace-services/internal/authn"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
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

func (m *MockLinkedAccountService) CreateOpenCosmosSessionService(w http.ResponseWriter, r *http.Request) {
	m.Called(w, r)
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

func TestStoreOpenCosmosSessionSecret(t *testing.T) {
	t.Parallel()

	namespace := "ws-test-workspace"
	secretName := openCosmosSecretName
	svc := &LinkedAccountService{
		K8sClient: fake.NewSimpleClientset(&corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{Name: namespace},
		}),
	}

	payload := OpenCosmosSessionPayload{
		AccessToken:  "access-token",
		RefreshToken: "refresh-token",
		ExpiresAt:    1780862369915,
		Scope:        "openid profile email data offline_access",
		TokenType:    "Bearer",
	}

	err := svc.storeOpenCosmosSessionSecret(payload, secretName, namespace)
	require.NoError(t, err)

	secret, err := svc.K8sClient.CoreV1().Secrets(namespace).Get(context.Background(), secretName, metav1.GetOptions{})
	require.NoError(t, err)
	require.Equal(t, payload.AccessToken, string(secret.Data["access_token"]))
	require.Equal(t, payload.RefreshToken, string(secret.Data["refresh_token"]))
	require.Equal(t, "1780862369915", string(secret.Data["expires_at"]))
	require.Equal(t, payload.Scope, string(secret.Data["scope"]))
	require.Equal(t, payload.TokenType, string(secret.Data["token_type"]))
	_, hasUserSub := secret.Data["user_sub"]
	require.False(t, hasUserSub)

	replacementPayload := OpenCosmosSessionPayload{
		AccessToken:  "replacement-access-token",
		RefreshToken: "replacement-refresh-token",
		ExpiresAt:    1780869999999,
		Scope:        "openid offline_access",
		TokenType:    "Bearer",
	}

	err = svc.storeOpenCosmosSessionSecret(replacementPayload, secretName, namespace)
	require.NoError(t, err)

	secrets, err := svc.K8sClient.CoreV1().Secrets(namespace).List(context.Background(), metav1.ListOptions{})
	require.NoError(t, err)
	require.Len(t, secrets.Items, 1)

	replacedSecret, err := svc.K8sClient.CoreV1().Secrets(namespace).Get(context.Background(), secretName, metav1.GetOptions{})
	require.NoError(t, err)
	require.Equal(t, replacementPayload.AccessToken, string(replacedSecret.Data["access_token"]))
	require.Equal(t, replacementPayload.RefreshToken, string(replacedSecret.Data["refresh_token"]))
	require.Equal(t, "1780869999999", string(replacedSecret.Data["expires_at"]))
	require.Equal(t, replacementPayload.Scope, string(replacedSecret.Data["scope"]))
}

func TestValidateAirbusLinkedAccountService_OpticalOrSAR(t *testing.T) {
	t.Parallel()

	hubAdminClaims := authn.Claims{}
	hubAdminClaims.RealmAccess.Roles = []string{"hub_admin"}

	tests := []struct {
		name            string
		opticalStatus   int
		opticalBody     string
		sarStatus       int
		sarBody         string
		wantHTTPStatus  int
		wantOptical     map[string]string
		wantSAR         bool
		skipBodyAsserts bool
	}{
		{
			name:            "both_upstream_fail",
			opticalStatus:   http.StatusInternalServerError,
			sarStatus:       http.StatusForbidden,
			wantHTTPStatus:  http.StatusInternalServerError,
			skipBodyAsserts: true,
		},
		{
			name:           "optical_ok_sar_fails",
			opticalStatus:  http.StatusOK,
			opticalBody:    `{"contracts":[{"contractId":"CTR1","name":"EODH_TEST"}]}`,
			sarStatus:      http.StatusForbidden,
			sarBody:        `{"code":0,"message":"account expired"}`,
			wantHTTPStatus: http.StatusOK,
			wantOptical:    map[string]string{"CTR1": "EODH_TEST"},
			wantSAR:        false,
		},
		{
			name:           "optical_fails_sar_ok_with_radar",
			opticalStatus:  http.StatusInternalServerError,
			sarStatus:      http.StatusOK,
			sarBody:        `{"services":["radar"]}`,
			wantHTTPStatus: http.StatusOK,
			wantOptical:    nil,
			wantSAR:        true,
		},
		{
			name:           "both_ok",
			opticalStatus:  http.StatusOK,
			opticalBody:    `{"contracts":[{"contractId":"C1","name":"N1"}]}`,
			sarStatus:      http.StatusOK,
			sarBody:        `{"services":["other","radar"]}`,
			wantHTTPStatus: http.StatusOK,
			wantOptical:    map[string]string{"C1": "N1"},
			wantSAR:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch {
				case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/token"):
					w.Header().Set("Content-Type", "application/json")
					_ = json.NewEncoder(w).Encode(map[string]string{"access_token": "test-access-token"})
				case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/contracts"):
					w.WriteHeader(tt.opticalStatus)
					if tt.opticalStatus == http.StatusOK {
						_, _ = w.Write([]byte(tt.opticalBody))
					}
				case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/whoami"):
					w.WriteHeader(tt.sarStatus)
					if tt.sarStatus == http.StatusOK {
						_, _ = w.Write([]byte(tt.sarBody))
					}
				default:
					w.WriteHeader(http.StatusNotFound)
				}
			}))
			t.Cleanup(srv.Close)

			svc := &LinkedAccountService{
				Config: &appconfig.Config{
					Providers: appconfig.ProvidersConfig{
						Airbus: appconfig.AirbusProviderConfig{
							AcessTokenURL:       srv.URL + "/token",
							OpticalContractsURL: srv.URL + "/contracts",
							SARContractsURL:     srv.URL + "/whoami",
						},
					},
				},
			}

			payload := []byte(`{"name":"airbus","key":"test-api-key"}`)
			req := httptest.NewRequest(http.MethodPost, "/workspaces/ws-airbus/linked-accounts/airbus/validate", bytes.NewReader(payload))
			req.Header.Set("Content-Type", "application/json")
			req = mux.SetURLVars(req, map[string]string{"workspace-id": "ws-airbus"})
			ctx := context.WithValue(req.Context(), middleware.ClaimsKey, hubAdminClaims)
			req = req.WithContext(ctx)

			rec := httptest.NewRecorder()
			svc.ValidateAirbusLinkedAccountService(rec, req)

			res := rec.Result()
			t.Cleanup(func() { _ = res.Body.Close() })

			require.Equal(t, tt.wantHTTPStatus, res.StatusCode)
			if tt.skipBodyAsserts {
				return
			}

			var got Payload
			require.NoError(t, json.NewDecoder(res.Body).Decode(&got))
			require.Equal(t, "airbus", got.Name)
			require.Equal(t, tt.wantOptical, got.Contracts.Optical)
			require.Equal(t, tt.wantSAR, got.Contracts.SAR)
		})
	}
}
