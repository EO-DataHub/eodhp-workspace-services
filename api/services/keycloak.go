package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/EO-DataHub/eodhp-workspace-services/models"
)

// KeycloakClient is a client for interacting with the Keycloak API.
type KeycloakClient struct {
	BaseURL      string
	ClientID     string
	ClientSecret string
	Realm        string
	Token        string
	HTTPClient   *http.Client
}

// NewKeycloakClient creates a new instance of KeycloakClient.
func NewKeycloakClient(baseURL, clientID, clientSecret, realm string) *KeycloakClient {
	return &KeycloakClient{
		BaseURL:      baseURL,
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Realm:        realm,
		HTTPClient:   &http.Client{},
	}
}

// GetToken retrieves a Keycloak access token using client_credentials.
func (kc *KeycloakClient) GetToken() error {
	tokenURL := fmt.Sprintf("%s/realms/%s/protocol/openid-connect/token", kc.BaseURL, kc.Realm)

	data := fmt.Sprintf("grant_type=client_credentials&client_id=%s&client_secret=%s", kc.ClientID, kc.ClientSecret)

	respBody, _, err := kc.makeRequest(http.MethodPost, tokenURL, "application/x-www-form-urlencoded", []byte(data))
	if err != nil {
		return err
	}

	// Parse the token from the response
	var tokenResponse struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(respBody, &tokenResponse); err != nil {
		return fmt.Errorf("failed to decode token response: %w", err)
	}

	kc.Token = tokenResponse.AccessToken
	return nil
}

// CreateGroup creates a new group in Keycloak.
func (kc *KeycloakClient) CreateGroup(groupName string) (int, error) {

	group := map[string]string{"name": groupName}
	body, _ := json.Marshal(group)

	url := fmt.Sprintf("%s/admin/realms/%s/groups", kc.BaseURL, kc.Realm)

	respBody, statusCode, err := kc.makeRequest(http.MethodPost, url, "application/json", body)
	if err != nil {
		return statusCode, err
	}

	if statusCode != http.StatusCreated {
		return statusCode, fmt.Errorf("failed to create group, status: %d, response: %s", statusCode, respBody)
	}

	return statusCode, nil
}

func (kc *KeycloakClient) GetGroup(groupName string) (*models.Group, error) {
	url := fmt.Sprintf("%s/admin/realms/%s/groups?search=%s", kc.BaseURL, kc.Realm, groupName)

	respBody, statusCode, err := kc.makeRequest(http.MethodGet, url, "application/json", nil)
	if err != nil {
		return nil, err
	}

	if statusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch group, status: %d", statusCode)
	}

	// Parse the response body into a slice of Group structs
	var groups []models.Group
	if err := json.Unmarshal(respBody, &groups); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Search for the group with the specified name
	for _, group := range groups {
		if group.Name == groupName {
			return &group, nil
		}
	}

	return nil, fmt.Errorf("group with name %s not found", groupName)
}

func (kc *KeycloakClient) GetGroupMembers(groupID string) ([]models.User, error) {
	url := fmt.Sprintf("%s/admin/realms/%s/groups/%s/members", kc.BaseURL, kc.Realm, groupID)

	respBody, statusCode, err := kc.makeRequest(http.MethodGet, url, "application/json", nil)
	if err != nil {
		return nil, err
	}

	// Check if the response status code is not OK
	if statusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch group members, status: %d", statusCode)
	}

	// Parse the response body into a slice of User structs
	var members []models.User
	if err := json.Unmarshal(respBody, &members); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return members, nil
}

func (kc *KeycloakClient) GetGroupMember(groupID, userID string) (*models.User, error) {

	members, err := kc.GetGroupMembers(groupID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch group members: %w", err)
	}

	// Search for the user in the list of members
	for _, member := range members {
		if member.ID == userID {
			return &member, nil
		}
	}

	// Return error if the user is not found
	return nil, fmt.Errorf("user with ID %s not found in group %s", userID, groupID)
}

// AddMemberToGroup adds a user to a group in Keycloak.
func (kc *KeycloakClient) AddMemberToGroup(userID, groupID string) error {
	url := fmt.Sprintf("%s/admin/realms/%s/users/%s/groups/%s", kc.BaseURL, kc.Realm, userID, groupID)

	respBody, statusCode, err := kc.makeRequest(http.MethodPut, url, "application/json", nil)
	if err != nil {
		return fmt.Errorf("failed to add member to group: %w", err)
	}

	if statusCode != http.StatusNoContent {
		return fmt.Errorf("failed to add member to group, status: %d, response: %s", statusCode, string(respBody))
	}

	return nil
}

// AddMemberToGroup adds a user to a group in Keycloak.
func (kc *KeycloakClient) RemoveMemberFromGroup(userID, groupID string) error {
	url := fmt.Sprintf("%s/admin/realms/%s/users/%s/groups/%s", kc.BaseURL, kc.Realm, userID, groupID)

	respBody, statusCode, err := kc.makeRequest(http.MethodDelete, url, "application/json", nil)
	if err != nil {
		return fmt.Errorf("failed to remove member from group: %w", err)
	}

	if statusCode != http.StatusNoContent {
		return fmt.Errorf("failed to remove member from group, status: %d, response: %s", statusCode, string(respBody))
	}

	return nil
}

// Helper function for making HTTP requests to keycloak API.
func (kc *KeycloakClient) makeRequest(method, url, contentType string, body []byte) ([]byte, int, error) {
	req, err := http.NewRequest(method, url, bytes.NewBuffer(body))
	if err != nil {
		return nil, 0, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", kc.Token))
	req.Header.Set("Content-Type", contentType)

	resp, err := kc.HTTPClient.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode >= 400 {
		return respBody, resp.StatusCode, fmt.Errorf("error response: status %d, body: %s", resp.StatusCode, string(respBody))
	}

	return respBody, resp.StatusCode, nil
}