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
func CreateWorkspace(workspaceDB *db.WorkspaceDB, publisher *events.EventPublisher) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {

		services.CreateWorkspaceService(workspaceDB, publisher, w, r)
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
