package services

import (
	"encoding/json"

	"net/http"

	"github.com/EO-DataHub/eodhp-workspace-services/api/middleware"
	"github.com/EO-DataHub/eodhp-workspace-services/db"
	"github.com/EO-DataHub/eodhp-workspace-services/internal/authn"
	"github.com/EO-DataHub/eodhp-workspace-services/models"
)

// GetWorkspacesService retrieves all workspaces accessible by the authenticated user's member groups.
func GetWorkspacesService(workspaceDB *db.WorkspaceDB, w http.ResponseWriter, r *http.Request) {

	// Extract member groups from user claims
	claims, ok := r.Context().Value(middleware.ClaimsKey).(authn.Claims)
	if !ok {
		http.Error(w, "Unauthorized: invalid claims", http.StatusUnauthorized)
		return
	}

	// Retrieve workspaces for the user based on their member groups
	workspaces, err := workspaceDB.GetUserWorkspaces(claims.MemberGroups)
	if err != nil {
		HandleErrResponse(w, http.StatusInternalServerError, err)
		return
	}

	// Send a success response with the retrieved workspaces data
	HandleSuccessResponse(w, http.StatusOK, nil, models.Response{
		Success: 1,
		Data:    models.WorkspacesResponse{Workspaces: workspaces},
	})

}

// CreateWorkspaceService handles creating a new workspace and publishing its creation event.
func CreateWorkspaceService(workspaceDB *db.WorkspaceDB, w http.ResponseWriter, r *http.Request) {

	// Decode the request body into a Workspace struct
	var messagePayload models.Workspace
	if err := json.NewDecoder(r.Body).Decode(&messagePayload); err != nil {
		workspaceDB.Log.Error().Err(err).Msg("Invalid request payload")
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	// Set a placeholder for MemberGroup (to be replaced by actual data from Keycloak)
	messagePayload.MemberGroup = "placeholder"

	// Begin the workspace creation transaction
	tx, err := workspaceDB.CreateWorkspace(&messagePayload)
	if err != nil {
		HandleErrResponse(w, http.StatusInternalServerError, err)
		return
	}

	// Publish a message for workspace creation
	err = workspaceDB.Events.Publish(messagePayload)
	if err != nil {
		// Rollback the transaction if publishing fails
		workspaceDB.Log.Error().Err(err).Msg("Failed to publish event.")
		tx.Rollback()
		http.Error(w, "Failed to create workspace event", http.StatusInternalServerError)
		return
	}

	// Commit the transaction after successful publishing
	if err := workspaceDB.CommitTransaction(tx); err != nil {
		http.Error(w, "Failed to commit workspace transaction", http.StatusInternalServerError)
		return
	}

	// Send a success response after creating the workspace and publishing the event
	HandleSuccessResponse(w, http.StatusCreated, nil, models.Response{
		Success: 1,
	})

}
