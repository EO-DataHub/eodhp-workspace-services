package handlers

import (
	"fmt"
	"log"
	"net/http"

	"github.com/EO-DataHub/eodhp-workspace-services/api/services"
	"github.com/EO-DataHub/eodhp-workspace-services/db"
	"github.com/EO-DataHub/eodhp-workspace-services/internal/events"
	"github.com/gorilla/mux"
	_ "github.com/lib/pq"
)

// InitializeKeycloakClient initializes the Keycloak client and retrieves the access token.
func InitializeKeycloakClient() (*services.KeycloakClient, error) {
	keycloakBaseURL := "https://dev.eodatahub.org.uk/keycloak"
	keycloakClientID := "oauth2-proxy-workspaces"
	keycloakClientSecret := "HWGhOvvqCn6Ts8aV7vRiETb8ht0OM78d"
	keycloakRealm := "eodhp"

	// Create a new Keycloak client
	keycloakClient := services.NewKeycloakClient(keycloakBaseURL, keycloakRealm)

	// Retrieve the token
	err := keycloakClient.GetToken(keycloakClientID, keycloakClientSecret)
	if err != nil {
		log.Printf("Failed to get Keycloak token: %v", err)
		return nil, fmt.Errorf("failed to authenticate with Keycloak: %w", err)
	}

	return keycloakClient, nil
}

// CreateWorkspace handles HTTP requests for creating a new workspace.
func CreateWorkspace(workspaceDB *db.WorkspaceDB, publisher *events.EventPublisher) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {

		// Initialize the Keycloak client for administrative tasks
		keycloakClient, err := InitializeKeycloakClient()
		if err != nil {
			log.Printf("Keycloak initialization error: %v", err)
			http.Error(w, "Failed to authenticate with Keycloak", http.StatusInternalServerError)
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
func GetUsers(workspaceDB *db.WorkspaceDB) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {

		// Initialize the Keycloak client for administrative tasks
		keycloakClient, err := InitializeKeycloakClient()
		if err != nil {
			log.Printf("Keycloak initialization error: %v", err)
			http.Error(w, "Failed to authenticate with Keycloak", http.StatusInternalServerError)
			return
		}

		services.GetUsersService(workspaceDB, keycloakClient, w, r)
	}
}

// AddUser handle HTTP requests for adding a user as a member of a workspace
func AddUser(workspaceDB *db.WorkspaceDB) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {

		// Initialize the Keycloak client for administrative tasks
		keycloakClient, err := InitializeKeycloakClient()
		if err != nil {
			log.Printf("Keycloak initialization error: %v", err)
			http.Error(w, "Failed to authenticate with Keycloak", http.StatusInternalServerError)
			return
		}

		services.AddUserService(workspaceDB, keycloakClient, w, r)
	}
}

// RemoveUser handle HTTP requests for removing a user as a member of a workspace
func RemoveUser(workspaceDB *db.WorkspaceDB) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {

		// Initialize the Keycloak client for administrative tasks
		keycloakClient, err := InitializeKeycloakClient()
		if err != nil {
			log.Printf("Keycloak initialization error: %v", err)
			http.Error(w, "Failed to authenticate with Keycloak", http.StatusInternalServerError)
			return
		}

		services.RemoveUserService(workspaceDB, keycloakClient, w, r)
	}
}
