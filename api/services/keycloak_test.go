package services

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetToken(t *testing.T) {
	mockResponse := `{"access_token": "mocked-access-token"}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/realms/test-realm/protocol/openid-connect/token", r.URL.Path)
		assert.Equal(t, "application/x-www-form-urlencoded", r.Header.Get("Content-Type"))
		_, _ = w.Write([]byte(mockResponse))
	}))
	defer server.Close()

	client := NewKeycloakClient(server.URL, "client-id", "client-secret", "test-realm")
	err := client.GetToken()
	assert.NoError(t, err)
	assert.Equal(t, "mocked-access-token", client.Token)
}

func TestCreateGroup(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/admin/realms/test-realm/groups", r.URL.Path)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		body, _ := io.ReadAll(r.Body)
		assert.JSONEq(t, `{"name": "test-group"}`, string(body))
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	client := NewKeycloakClient(server.URL, "client-id", "client-secret", "test-realm")
	client.Token = "mocked-token"

	statusCode, err := client.CreateGroup("test-group")
	assert.NoError(t, err)
	assert.Equal(t, http.StatusCreated, statusCode)
}

func TestGetGroup(t *testing.T) {
	mockResponse := `[{"id": "group-id", "name": "test-group"}]`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/admin/realms/test-realm/groups", r.URL.Path)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		_, _ = w.Write([]byte(mockResponse))
	}))
	defer server.Close()

	client := NewKeycloakClient(server.URL, "client-id", "client-secret", "test-realm")
	client.Token = "mocked-token"

	group, err := client.GetGroup("test-group")
	assert.NoError(t, err)
	assert.Equal(t, "group-id", group.ID)
	assert.Equal(t, "test-group", group.Name)
}

func TestGetGroupMembers(t *testing.T) {
	mockResponse := `[{"id": "user1", "username": "user1"}, {"id": "user2", "username": "user2"}]`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/admin/realms/test-realm/groups/group-id/members", r.URL.Path)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		_, _ = w.Write([]byte(mockResponse))
	}))
	defer server.Close()

	client := NewKeycloakClient(server.URL, "client-id", "client-secret", "test-realm")
	client.Token = "mocked-token"

	members, err := client.GetGroupMembers("group-id")
	assert.NoError(t, err)
	assert.Len(t, members, 2)
	assert.Equal(t, "user1", members[0].ID)
	assert.Equal(t, "user2", members[1].ID)
}

func TestAddMemberToGroup(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/admin/realms/test-realm/users/user-id/groups/group-id", r.URL.Path)
		assert.Equal(t, http.MethodPut, r.Method)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := NewKeycloakClient(server.URL, "client-id", "client-secret", "test-realm")
	client.Token = "mocked-token"

	err := client.AddMemberToGroup("user-id", "group-id")
	assert.NoError(t, err)
}

func TestRemoveMemberFromGroup(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/admin/realms/test-realm/users/user-id/groups/group-id", r.URL.Path)
		assert.Equal(t, http.MethodDelete, r.Method)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := NewKeycloakClient(server.URL, "client-id", "client-secret", "test-realm")
	client.Token = "mocked-token"

	err := client.RemoveMemberFromGroup("user-id", "group-id")
	assert.NoError(t, err)
}
