package services

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/EO-DataHub/eodhp-workspace-services/db"
	"github.com/EO-DataHub/eodhp-workspace-services/internal/events"
	"github.com/EO-DataHub/eodhp-workspace-services/models"
)

// Handles the creation of a workspace and its related components
func CreateWorkspaceService(workspaceDB *db.WorkspaceDB, w http.ResponseWriter, r *http.Request) {
	
	// Parse and decode the request body into a WorkspaceRequest object
	var workspaceRequest models.WorkspaceRequest
	if err := json.NewDecoder(r.Body).Decode(&workspaceRequest); err != nil {
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		log.Println("Error decoding request body:", err)
		return
	}

	// Extract the workspace data from the request
	workspace := models.Workspace{
		Name:               workspaceRequest.Name,
		Namespace:          workspaceRequest.Namespace,
		ServiceAccountName: workspaceRequest.ServiceAccountName,
		AWSRoleName:        workspaceRequest.AWSRoleName,
	}

	// Insert the workspace and related data into the database
	workspaceID, err := workspaceDB.InsertWorkspace(
		workspace,
		workspaceRequest.EFSAccessPoint,
		workspaceRequest.S3Buckets,
		workspaceRequest.PersistentVolumes,
		workspaceRequest.PersistentVolumeClaims,
	)

	if err != nil {
		// Respond with error
		http.Error(w, "Failed to create workspace", http.StatusInternalServerError)
		log.Println("Error inserting workspace:", err)
		return
	}

	// Respond with success
	w.WriteHeader(http.StatusCreated)
	w.Write([]byte("Workspace created successfully"))

	// After inserting workspace, notify the event system
	if workspaceDB.Events != nil {
		eventPayload := events.EventPayload{
			WorkspaceID: workspaceID, // Assuming the Workspace struct has an ID field
			Action:      "create",
		}

		// Send notification via the event system
		err = workspaceDB.Events.Notify(eventPayload)
		if err != nil {
			workspaceDB.Log.Error().Err(err).Msg("Failed to send event notification")
			return
		}
	}
}
