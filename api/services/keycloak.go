package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// KeycloakClient is a client for interacting with the Keycloak API.
type KeycloakClient struct {
	BaseURL    string
	Realm      string
	Token      string
	HTTPClient *http.Client
}

// NewKeycloakClient creates a new instance of KeycloakClient.
func NewKeycloakClient(baseURL, realm string) *KeycloakClient {
	return &KeycloakClient{
		BaseURL:    baseURL,
		Realm:      realm,
		HTTPClient: &http.Client{},
	}
}

// GetToken retrieves a Keycloak access token using client_credentials.
func (kc *KeycloakClient) GetToken(clientID, clientSecret string) error {
	tokenURL := fmt.Sprintf("%s/realms/%s/protocol/openid-connect/token", kc.BaseURL, kc.Realm)

	// Prepare form data
	data := fmt.Sprintf("grant_type=client_credentials&client_id=%s&client_secret=%s", clientID, clientSecret)
	req, err := http.NewRequest(http.MethodPost, tokenURL, bytes.NewBufferString(data))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	// Make the HTTP request
	resp, err := kc.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	// Check for successful response
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to get token, status: %d, response: %s", resp.StatusCode, string(body))
	}

	// Parse the token from the response
	var tokenResponse struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tokenResponse); err != nil {
		return fmt.Errorf("failed to decode token response: %w", err)
	}

	kc.Token = tokenResponse.AccessToken

	return nil
}

// CreateGroup creates a new group in Keycloak.
func (kc *KeycloakClient) CreateGroup(groupName string) (string, error) {
	group := map[string]string{"name": groupName}
	body, _ := json.Marshal(group)

	url := fmt.Sprintf("%s/admin/realms/%s/groups", kc.BaseURL, kc.Realm)
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(body))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", kc.Token))
	req.Header.Set("Content-Type", "application/json")

	resp, err := kc.HTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("failed to create group, status: %d, response: %s", resp.StatusCode, string(body))
	}

	location := resp.Header.Get("Location")
	return extractIDFromLocation(location), nil
}

// Helper function to extract group ID from the Location header.
func extractIDFromLocation(location string) string {
	parts := bytes.Split([]byte(location), []byte("/"))
	return string(parts[len(parts)-1])
}
