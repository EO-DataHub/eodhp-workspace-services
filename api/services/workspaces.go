package services

import (
	"encoding/json"
	"fmt"

	"net/http"

	ws_manager "github.com/EO-DataHub/eodhp-workspace-manager/models"
	"github.com/EO-DataHub/eodhp-workspace-services/api/middleware"
	"github.com/EO-DataHub/eodhp-workspace-services/db"
	"github.com/EO-DataHub/eodhp-workspace-services/internal/authn"
	"github.com/EO-DataHub/eodhp-workspace-services/internal/events"
	ws_services "github.com/EO-DataHub/eodhp-workspace-services/models"
	"github.com/gorilla/mux"
	"github.com/rs/zerolog/log"
)

// GetWorkspacesService retrieves all workspaces accessible to the authenticated user's groups.
func GetWorkspacesService(workspaceDB *db.WorkspaceDB, w http.ResponseWriter, r *http.Request) {

	// Extract groups the user is a member of from the claims
	claims, ok := r.Context().Value(middleware.ClaimsKey).(authn.Claims)
	if !ok {
		http.Error(w, "Unauthorized: invalid claims", http.StatusUnauthorized)
		return
	}

	// Retrieve workspaces assigned to these groups
	workspaces, err := workspaceDB.GetUserWorkspaces(claims.MemberGroups)
	if err != nil {
		HandleErrResponse(w, http.StatusInternalServerError, err)
		return
	}

	// Send a success response with the retrieved workspaces data
	HandleSuccessResponse(w, http.StatusOK, nil, ws_services.Response{
		Success: 1,
		Data:    ws_services.WorkspacesResponse{Workspaces: workspaces},
	}, "")

}

// GetWorkspaceService retrieves an individual workspace accessible to the authenticated user's groups.
func GetWorkspaceService(workspaceDB *db.WorkspaceDB, w http.ResponseWriter, r *http.Request) {

	// Extract groups the user is a member of from the claims
	claims, ok := r.Context().Value(middleware.ClaimsKey).(authn.Claims)
	if !ok {
		http.Error(w, "Unauthorized: invalid claims", http.StatusUnauthorized)
		return
	}

	// Parse the workspace ID from the URL path
	workspaceID := mux.Vars(r)["workspace-id"]

	// Retrieve account associated with the user's username
	workspace, err := workspaceDB.GetWorkspace(workspaceID)

	if err != nil {
		HandleErrResponse(w, http.StatusInternalServerError, err)
		return
	}

	// Check if the account owner matches any of the claims member groups
	if !isMemberGroupAuthorized(workspace.MemberGroup, claims.MemberGroups) {
		// Return a success: 0 response to indicate unauthorized access without exposing details
		HandleSuccessResponse(w, http.StatusForbidden, nil, ws_services.Response{
			Success:      0,
			ErrorCode:    "unauthorized",
			ErrorDetails: "You do not have access to this account.",
		}, "")
		return
	}

	// Send a success response with the retrieved workspaces data
	HandleSuccessResponse(w, http.StatusOK, nil, ws_services.Response{
		Success: 1,
		Data:    ws_services.WorkspaceResponse{Workspace: *workspace},
	}, "")
}

// CreateWorkspaceService handles creating a new workspace and publishing its creation event.
func CreateWorkspaceService(workspaceDB *db.WorkspaceDB, publisher *events.EventPublisher, w http.ResponseWriter, r *http.Request) {

	// Decode the request body into a Workspace struct
	var messagePayload ws_manager.WorkspaceSettings
	if err := json.NewDecoder(r.Body).Decode(&messagePayload); err != nil {
		log.Error().Err(err).Msg("Invalid request payload")
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	// Check the name is DNS-compatible
	if !IsDNSCompatible(messagePayload.Name) {
		log.Error().Msg("Invalid workspace name: must be DNS-compatible")
		HandleErrResponse(w, http.StatusConflict, fmt.Errorf("invalid workspace name: must be DNS-compatible"))
		return
	}

	// Check that the workspace name does not already exist
	workspaceExists, err := workspaceDB.CheckWorkspaceExists(messagePayload.Name)
	if err != nil {
		HandleErrResponse(w, http.StatusInternalServerError, err)
		return
	}

	// Return a conflict response if the workspace name already exists
	if workspaceExists {
		log.Error().Msgf("workspace with name %s already exists", messagePayload.Name)
		HandleErrResponse(w, http.StatusConflict, fmt.Errorf("workspace with name %s already exists", messagePayload.Name))
		return
	}

	// Check that the account exists and the user is the account owner
	account, err := workspaceDB.CheckAccountExists(messagePayload.Account)
	if err != nil {
		fmt.Println(err)
		HandleErrResponse(w, http.StatusInternalServerError, err)
		return
	}

	// Return a not found response if the account does not exist
	if !account {
		log.Error().Msgf("account with ID %s not found", messagePayload.Account)
		HandleErrResponse(w, http.StatusNotFound, fmt.Errorf("account with ID %s not found", messagePayload.Account))
		return
	}

	messagePayload.Status = "creating"

	// Begin the workspace creation transaction
	tx, err := workspaceDB.CreateWorkspace(&messagePayload)
	if err != nil {
		HandleErrResponse(w, http.StatusInternalServerError, err)
		return
	}

	// Publish a message for workspace creation
	err = publisher.Publish(messagePayload)
	if err != nil {
		// Rollback the transaction if publishing fails
		log.Error().Err(err).Msg("Failed to publish event.")
		tx.Rollback()
		http.Error(w, "Failed to create workspace event", http.StatusInternalServerError)
		return
	}

	// Commit the transaction after successful publishing
	if err := workspaceDB.CommitTransaction(tx); err != nil {
		http.Error(w, "Failed to commit workspace transaction", http.StatusInternalServerError)
		return
	}

	// Add location header
	var location = fmt.Sprintf("%s/%s", r.URL.Path, messagePayload.ID)

	// Send a success response after creating the workspace and publishing the event
	HandleSuccessResponse(w, http.StatusCreated, nil, ws_services.Response{
		Success: 1,
		Data:    ws_services.WorkspaceResponse{Workspace: messagePayload},
	}, location)

}
