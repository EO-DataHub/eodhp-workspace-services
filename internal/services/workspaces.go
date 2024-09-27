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
func CreateWorkspaceService(w http.ResponseWriter, r *http.Request) {
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

	// Connect to the database
	dbConn, err := db.ConnectPostgres()
	if err != nil {
		http.Error(w, "Failed to connect to the database", http.StatusInternalServerError)
		log.Println("Database connection error:", err)
		return
	}
	defer dbConn.Close()

	// Insert the workspace and related data into the database
	workspaceID, err := db.InsertWorkspaceWithRelatedData(
		dbConn,
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

	// Use the workspaceID in the Pulsar message
	err = events.PublishEvent(workspaceID, "create")
	if err != nil {
		log.Printf("Failed to publish create event: %v", err)
	}

	// Respond with success
	w.WriteHeader(http.StatusCreated)
	w.Write([]byte("Workspace created successfully"))
}
