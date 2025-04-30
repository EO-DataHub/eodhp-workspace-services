package handlers

import (
	"net/http"

	ws_manager "github.com/EO-DataHub/eodhp-workspace-manager/models"
	"github.com/EO-DataHub/eodhp-workspace-services/api/services"
	"github.com/gorilla/mux"
	_ "github.com/lib/pq"
)

// Trick compiler to keep import for swag annotation
var _ = ws_manager.WorkspaceSettings{}

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

// @Summary Get a list of workspaces you are a member of
// @Description Retrieve a list of workspaces for the authenticated user.
// @Tags Workspaces
// @Accept json
// @Produce json
// @Success 200 {array} ws_manager.WorkspaceSettings
// @Failure 401 {object} string
// @Failure 500 {object} string
// @Router /workspaces [get]
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

// @Summary Get a workspace by ID
// @Description Retrieve a specific workspace using its ID for the authenticated user.
// @Tags Workspaces
// @Accept json
// @Produce json
// @Param workspace-id path string true "Workspace ID" // Workspace ID from the URL path
// @Success 200 {object} ws_manager.WorkspaceSettings
// @Failure 400 {object} string
// @Failure 401 {object} string
// @Failure 500 {object} string
// @Router /workspaces/{workspace-id} [get]
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

// @Summary Get users of a workspace
// @Description Retrieve a list of users who are members of the specified workspace.
// @Tags Workspaces
// @Accept json
// @Produce json
// @Param workspace-id path string true "Workspace ID"
// @Success 200 {array} models.User "List of users"
// @Failure 400 {object} string
// @Failure 401 {object} string
// @Failure 500 {object} string
// @Router /workspaces/{workspace-id}/users [get]
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

// @Summary Get a user of a workspace
// @Description Retrieve details of a specific user that is a member of the specified workspace.
// @Tags Workspaces
// @Accept json
// @Produce json
// @Param workspace-id path string true "Workspace ID"
// @Param username path string true "Username"
// @Success 200 {object} models.User "User details"
// @Failure 400 {object} string
// @Failure 401 {object} string
// @Failure 404 {object} string
// @Failure 500 {object} string
// @Router /workspaces/{workspace-id}/users/{username} [get]
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

// @Summary Add a user to a workspace
// @Description Add a user to the specified workspace by providing the workspace ID and username.
// @Tags Workspaces
// @Accept json
// @Produce json
// @Param workspace-id path string true "Workspace ID"
// @Param username path string true "Username"
// @Success 200 {string} string "User successfully added to the workspace"
// @Failure 400 {object} string "Bad Request"
// @Failure 401 {object} string "Unauthorized"
// @Failure 404 {object} string "Workspace or User Not Found"
// @Failure 500 {object} string "Internal Server Error"
// @Router /workspaces/{workspace-id}/users/{username} [put]
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

// @Summary Remove a user from a workspace
// @Description Remove a user from the specified workspace by providing the workspace ID and username.
// @Tags Workspaces
// @Accept json
// @Produce json
// @Param workspace-id path string true "Workspace ID"
// @Param username path string true "Username"
// @Success 200 {string} string "User successfully removed from the workspace"
// @Failure 400 {object} string "Bad Request"
// @Failure 401 {object} string "Unauthorized"
// @Failure 404 {object} string "Workspace or User Not Found"
// @Failure 500 {object} string "Internal Server Error"
// @Router /workspaces/{workspace-id}/users/{username} [delete]
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
