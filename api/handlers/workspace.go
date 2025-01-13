package handlers

import (
	"net/http"

	"github.com/EO-DataHub/eodhp-workspace-services/api/services"
	"github.com/EO-DataHub/eodhp-workspace-services/db"
	"github.com/EO-DataHub/eodhp-workspace-services/internal/events"
	"github.com/gorilla/mux"
	_ "github.com/lib/pq"
)

// CreateWorkspace handles HTTP requests for creating a new workspace.
func CreateWorkspace(workspaceDB *db.WorkspaceDB, publisher *events.EventPublisher, keycloakClient *services.KeycloakClient) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {

		// Get a token from keycloak so we can interact with it's API
		err := keycloakClient.GetToken()
		if err != nil {
			http.Error(w, "Authentication failed.", http.StatusInternalServerError)
			return
		}

		services.CreateWorkspaceService(workspaceDB, publisher, keycloakClient, w, r)
	}
}

// GetWorkspaces handles HTTP requests for retrieving workspaces.
func GetWorkspaces(workspaceDB *db.WorkspaceDB) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {
		services.GetWorkspacesService(workspaceDB, w, r)
	}
}

// GetWorkspace handles HTTP requests for retrieving an individual workspace.
func GetWorkspace(workspaceDB *db.WorkspaceDB) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {
		services.GetWorkspaceService(workspaceDB, w, r)
	}
}

// UpdateWorkspace handles HTTP requests for updating a specific workspace by ID.
// This is a placeholder for the actual implementation.
func UpdateWorkspace(workspaceDB *db.WorkspaceDB) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {

		vars := mux.Vars(r)
		workspaceID := vars["workspace-id"]

		// placeholder for the implementation
		http.Error(w, "Failed to update workspace "+workspaceID, http.StatusInternalServerError)
	}
}

// PatchWorkspace handles HTTP requests for partially updating a specific workspace by ID.
// This is a placeholder for the actual implementation.
func PatchWorkspace(workspaceDB *db.WorkspaceDB) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {

		vars := mux.Vars(r)
		workspaceID := vars["workspace-id"]

		// placeholder for the implementation
		http.Error(w, "Failed to patch workspace "+workspaceID, http.StatusInternalServerError)
	}
}

// GetUsers handles HTTP requests for retrieving users that are members of a workspace
func GetUsers(workspaceDB *db.WorkspaceDB, keycloakClient *services.KeycloakClient) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {

		// Get a token from keycloak so we can interact with it's API
		err := keycloakClient.GetToken()
		if err != nil {
			http.Error(w, "Authentication failed.", http.StatusInternalServerError)
			return
		}

		services.GetUsersService(workspaceDB, keycloakClient, w, r)
	}
}

// GetUser handles HTTP requests for retrieving individual users that are members of a workspace
func GetUser(workspaceDB *db.WorkspaceDB, keycloakClient *services.KeycloakClient) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {

		// Get a token from keycloak so we can interact with it's API
		err := keycloakClient.GetToken()
		if err != nil {
			http.Error(w, "Authentication failed.", http.StatusInternalServerError)
			return
		}

		services.GetUserService(workspaceDB, keycloakClient, w, r)
	}
}

// AddUser handle HTTP requests for adding a user as a member of a workspace
func AddUser(workspaceDB *db.WorkspaceDB, keycloakClient *services.KeycloakClient) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {

		// Get a token from keycloak so we can interact with it's API
		err := keycloakClient.GetToken()
		if err != nil {
			http.Error(w, "Authentication failed.", http.StatusInternalServerError)
			return
		}

		services.AddUserService(workspaceDB, keycloakClient, w, r)
	}
}

// RemoveUser handle HTTP requests for removing a user as a member of a workspace
func RemoveUser(workspaceDB *db.WorkspaceDB, keycloakClient *services.KeycloakClient) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {

		// Get a token from keycloak so we can interact with it's API
		err := keycloakClient.GetToken()
		if err != nil {
			http.Error(w, "Authentication failed.", http.StatusInternalServerError)
			return
		}

		services.RemoveUserService(workspaceDB, keycloakClient, w, r)
	}
}
