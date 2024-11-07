package services

import (
	"encoding/json"

	"net/http"

	"github.com/EO-DataHub/eodhp-workspace-services/api/middleware"
	"github.com/EO-DataHub/eodhp-workspace-services/db"
	"github.com/EO-DataHub/eodhp-workspace-services/internal/authn"
	"github.com/EO-DataHub/eodhp-workspace-services/models"
)

func GetWorkspacesService(workspaceDB *db.WorkspaceDB, w http.ResponseWriter, r *http.Request) {

	// Extract the memberGroups from the claims
	claims, ok := r.Context().Value(middleware.ClaimsKey).(authn.Claims)
	if !ok {
		http.Error(w, "Unauthorized: invalid claims", http.StatusUnauthorized)
		return
	}

	// Retrieve user workspaces based on the memberGroups
	workspaces, err := workspaceDB.GetUserWorkspaces(claims.MemberGroups)
	if err != nil {
		HandleErrResponse(w, http.StatusInternalServerError, err)
		return
	}

	// Encode and send the response
	HandleSuccessResponse(w, http.StatusOK, nil, models.Response{
		Success: 1,
		Data:    models.WorkspacesResponse{Workspaces: workspaces},
	})

}

// Handles the creation of a workspace and its related components
func CreateWorkspaceService(workspaceDB *db.WorkspaceDB, w http.ResponseWriter, r *http.Request) {

	// Parse and decode the request body into a MessagePayload object
	var messagePayload models.Workspace
	if err := json.NewDecoder(r.Body).Decode(&messagePayload); err != nil {
		workspaceDB.Log.Error().Err(err).Msg("Invalid request payload")
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	// Add the claims to the message payload
	messagePayload.MemberGroup = "placeholder" // TODO: will be replaced with the actual member group from Keycloak

	// Create the workspace transaction
	tx, err := workspaceDB.InsertWorkspace(&messagePayload)
	if err != nil {
		HandleErrResponse(w, http.StatusInternalServerError, err)
		return
	}

	// Publish the message to be picked up by the workspace manager
	err = workspaceDB.Events.Publish(messagePayload)
	if err != nil {
		workspaceDB.Log.Error().Err(err).Msg("Failed to publish event.")

		tx.Rollback() // Rollback the transaction if the event fails to publish
		http.Error(w, "Failed to create workspace event", http.StatusInternalServerError)
		return
	}

	// Commit the transaction after sucessfully publishing the event
	if err := workspaceDB.CommitTransaction(tx); err != nil {
		http.Error(w, "Failed to commit workspace transaction", http.StatusInternalServerError)
		return
	}

	HandleSuccessResponse(w, http.StatusCreated, nil, models.Response{
		Success: 1,
	})

}
