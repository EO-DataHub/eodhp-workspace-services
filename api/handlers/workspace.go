package handlers

import (
	"net/http"

	"github.com/EO-DataHub/eodhp-workspace-services/api/services"
	"github.com/gorilla/mux"
	_ "github.com/lib/pq"
)

// CreateWorkspace handles HTTP requests for creating a new workspace.
func CreateWorkspace(svc *services.WorkspaceService) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {

		// Get a token from keycloak so we can interact with it's API
		err := svc.KC.GetToken()
		if err != nil {
			http.Error(w, "Authentication failed.", http.StatusInternalServerError)
			return
		}

		svc.CreateWorkspaceService(w, r)
	}
}


// GetWorkspaces handles HTTP requests for retrieving workspaces.
func GetWorkspaces(svc *services.WorkspaceService) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {

		// Get a token from keycloak so we can interact with it's API
		err := svc.KC.GetToken()
		if err != nil {
			http.Error(w, "Authentication failed.", http.StatusInternalServerError)
			return
		}

		svc.GetWorkspacesService(w, r)
	}
}

// GetWorkspace handles HTTP requests for retrieving an individual workspace.
func GetWorkspace(svc *services.WorkspaceService) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {

		// Get a token from keycloak so we can interact with it's API
		err := svc.KC.GetToken()
		if err != nil {
			http.Error(w, "Authentication failed.", http.StatusInternalServerError)
			return
		}

		svc.GetWorkspaceService(w, r)
	}
}

// DeleteWorkspace handles HTTP requests for deleting a workspace
func DeleteWorkspace(svc *services.WorkspaceService) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {

		// Get a token from keycloak so we can interact with it's API
		err := svc.KC.GetToken()
		if err != nil {
			http.Error(w, "Authentication failed.", http.StatusInternalServerError)
			return
		}

		svc.DeleteWorkspaceService(w, r)
	}
}


// UpdateWorkspace handles HTTP requests for updating a specific workspace by ID.
// This is a placeholder for the actual implementation.
func UpdateWorkspace(svc *services.WorkspaceService) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {

		vars := mux.Vars(r)
		workspaceID := vars["workspace-id"]

		// placeholder for the implementation
		http.Error(w, "Failed to update workspace "+workspaceID, http.StatusInternalServerError)
	}
}

// PatchWorkspace handles HTTP requests for partially updating a specific workspace by ID.
// This is a placeholder for the actual implementation.
func PatchWorkspace(svc *services.WorkspaceService) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {

		vars := mux.Vars(r)
		workspaceID := vars["workspace-id"]

		// placeholder for the implementation
		http.Error(w, "Failed to patch workspace "+workspaceID, http.StatusInternalServerError)
	}
}

// GetUsers handles HTTP requests for retrieving users that are members of a workspace
func GetUsers(svc *services.WorkspaceService) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {

		// Get a token from keycloak so we can interact with it's API
		err := svc.KC.GetToken()
		if err != nil {
			http.Error(w, "Authentication failed.", http.StatusInternalServerError)
			return
		}

		svc.GetUsersService(w, r)
	}
}

// GetUser handles HTTP requests for retrieving individual users that are members of a workspace
func GetUser(svc *services.WorkspaceService) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {

		// Get a token from keycloak so we can interact with it's API
		err := svc.KC.GetToken()
		if err != nil {
			http.Error(w, "Authentication failed.", http.StatusInternalServerError)
			return
		}

		svc.GetUserService(w, r)
	}
}

// AddUser handle HTTP requests for adding a user as a member of a workspace
func AddUser(svc *services.WorkspaceService) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {

		// Get a token from keycloak so we can interact with it's API
		err := svc.KC.GetToken()
		if err != nil {
			http.Error(w, "Authentication failed.", http.StatusInternalServerError)
			return
		}

		svc.AddUserService(w, r)
	}
}

// RemoveUser handle HTTP requests for removing a user as a member of a workspace
func RemoveUser(svc *services.WorkspaceService) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {

		// Get a token from keycloak so we can interact with it's API
		err := svc.KC.GetToken()
		if err != nil {
			http.Error(w, "Authentication failed.", http.StatusInternalServerError)
			return
		}

		svc.RemoveUserService(w, r)
	}
}
