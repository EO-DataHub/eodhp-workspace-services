package handlers

import (
	"net/http"

	"github.com/EO-DataHub/eodhp-workspace-services/db"
	"github.com/EO-DataHub/eodhp-workspace-services/internal/services"
	"github.com/gorilla/mux"
	_ "github.com/lib/pq"
)

func CreateWorkspace(workspaceDB *db.WorkspaceDB) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {

		services.CreateWorkspaceService(workspaceDB, w, r)
	}
}

func GetWorkspaces(workspaceDB *db.WorkspaceDB) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {
		// placeholder for the implementation
		http.Error(w, "Failed to get workspaces", http.StatusInternalServerError)
	}
}

func UpdateWorkspace(workspaceDB *db.WorkspaceDB) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {

		vars := mux.Vars(r)
		workspaceID := vars["workspace-id"]

		// placeholder for the implementation
		http.Error(w, "Failed to update workspace "+workspaceID, http.StatusInternalServerError)
	}
}

func PatchWorkspace(workspaceDB *db.WorkspaceDB) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {

		vars := mux.Vars(r)
		workspaceID := vars["workspace-id"]

		// placeholder for the implementation
		http.Error(w, "Failed to patch workspace "+workspaceID, http.StatusInternalServerError)
	}
}
