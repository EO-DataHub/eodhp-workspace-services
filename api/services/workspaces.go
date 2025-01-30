package services

import (
	"encoding/json"
	"fmt"

	"net/http"

	ws_manager "github.com/EO-DataHub/eodhp-workspace-manager/models"
	"github.com/EO-DataHub/eodhp-workspace-services/api/middleware"
	"github.com/EO-DataHub/eodhp-workspace-services/internal/authn"
	"github.com/gorilla/mux"
	"github.com/rs/zerolog/log"
)

// GetWorkspacesService retrieves all workspaces accessible to the authenticated user's groups.
func GetWorkspacesService(svc *Service, w http.ResponseWriter, r *http.Request) {

	// Extract groups the user is a member of from the claims
	claims, ok := r.Context().Value(middleware.ClaimsKey).(authn.Claims)
	if !ok {
		WriteResponse(w, http.StatusUnauthorized, nil)
		return
	}

	// Retrieve workspaces assigned to these groups
	workspaces, err := svc.DB.GetUserWorkspaces(claims.MemberGroups)
	if err != nil {
		WriteResponse(w, http.StatusInternalServerError, nil)
		return
	}

	// Send a success response with the retrieved workspaces data
	WriteResponse(w, http.StatusOK, workspaces)

}

// GetWorkspaceService retrieves an individual workspace accessible to the authenticated user's groups.
func GetWorkspaceService(svc *Service, w http.ResponseWriter, r *http.Request) {

	// logger := log.Ctx(r.Context())
	// logger.Info().Send()
	// Extract groups the user is a member of from the claims
	claims, ok := r.Context().Value(middleware.ClaimsKey).(authn.Claims)
	if !ok {
		WriteResponse(w, http.StatusUnauthorized, nil)
		return
	}

	// Parse the workspace ID from the URL path
	workspaceID := mux.Vars(r)["workspace-id"]
	// logger.Info().Str("workspace_id", workspaceID).Msg("Fetching workspace")

	// Retrieve account associated with the user's username
	workspace, err := svc.DB.GetWorkspace(workspaceID)

	if err != nil {
		log.Error().Err(err).Send()
		WriteResponse(w, http.StatusNotFound, "Workspace does not exist.")
		return
	}

	// Check if the account owner matches any of the claims member groups
	if !isMemberGroupAuthorized(workspace.MemberGroup, claims.MemberGroups) {
		WriteResponse(w, http.StatusForbidden, nil)
		return
	}

	// Send a success response with the retrieved workspaces data
	WriteResponse(w, http.StatusOK, *workspace)
}

// CreateWorkspaceService handles creating a new workspace and publishing its creation event.
func CreateWorkspaceService(svc *Service, w http.ResponseWriter, r *http.Request) {

	// Extract the claims to get the users KC ID
	claims, ok := r.Context().Value(middleware.ClaimsKey).(authn.Claims)
	if !ok {
		WriteResponse(w, http.StatusUnauthorized, nil)
		return
	}

	accountOwnerID := claims.Subject

	// Decode the request body into a Workspace struct
	var messagePayload ws_manager.WorkspaceSettings
	if err := json.NewDecoder(r.Body).Decode(&messagePayload); err != nil {
		log.Error().Err(err).Msg("Invalid request payload")
		WriteResponse(w, http.StatusBadRequest, nil)
		return
	}

	// Check the name is DNS-compatible
	if !IsDNSCompatible(messagePayload.Name) {
		WriteResponse(w, http.StatusConflict, fmt.Errorf("invalid workspace name: must be DNS-compatible"))
		return
	}

	// Check that the workspace name does not already exist
	workspaceExists, err := svc.DB.CheckWorkspaceExists(messagePayload.Name)
	if err != nil {
		WriteResponse(w, http.StatusInternalServerError, nil)
		return
	}

	// Return a conflict response if the workspace name already exists
	if workspaceExists {
		log.Error().Msgf("workspace with name %s already exists", messagePayload.Name)
		WriteResponse(w, http.StatusConflict, fmt.Errorf("workspace with name %s already exists", messagePayload.Name))
		return
	}

	// Check that the account exists and the user is the account owner
	account, err := svc.DB.CheckAccountExists(messagePayload.Account)
	if err != nil {
		WriteResponse(w, http.StatusInternalServerError, nil)
		return
	}

	// Return a not found response if the account does not exist
	if !account {
		log.Error().Msgf("account with ID %s not found", messagePayload.Account)
		WriteResponse(w, http.StatusNotFound, fmt.Errorf("The account associated with this workspace does not exist"))
		return
	}

	// Create a group in Keycloak - the group name is the same as the workspace name
	messagePayload.MemberGroup = messagePayload.Name
	statusCode, err := svc.KC.CreateGroup(messagePayload.MemberGroup)

	if err != nil {
		WriteResponse(w, statusCode, nil)
		return
	}

	log.Info().Msgf("Group %s created successfully", messagePayload.MemberGroup)

	// Find the group ID just created from keycloak
	group, err := svc.KC.GetGroup(messagePayload.MemberGroup)

	if err != nil {
		WriteResponse(w, http.StatusInternalServerError, nil)
		return
	}

	err = svc.KC.AddMemberToGroup(accountOwnerID, group.ID)
	if err != nil {
		log.Error().Err(err).Msg("Failed to add member to group")
		WriteResponse(w, http.StatusInternalServerError, nil)
		return
	}

	// Begin the workspace creation transaction
	messagePayload.Status = "creating"

	// Define default object and block stores
	messagePayload.Stores = &[]ws_manager.Stores{
		// {
		// 	Object: []ws_manager.ObjectStore{
		// 		{Name: messagePayload.Name + "-object-store"},
		// 	},
		// 	Block: []ws_manager.BlockStore{
		// 		{Name: messagePayload.Name + "-block-store"},
		// 	},
		// },
		{
			Object: []ws_manager.ObjectStore{
				{Name: messagePayload.Name + "-object-store"},
			},
			Block: []ws_manager.BlockStore{
				{Name: "block-store"},
			},
		},
	}
	tx, err := svc.DB.CreateWorkspace(&messagePayload)
	if err != nil {
		WriteResponse(w, http.StatusInternalServerError, nil)
		return
	}

	// Publish a message for workspace creation
	err = svc.Publisher.Publish(messagePayload)
	if err != nil {
		// Rollback the transaction if publishing fails
		log.Error().Err(err).Msg("Failed to publish event.")
		tx.Rollback()
		WriteResponse(w, http.StatusInternalServerError, nil)
		return
	}

	// Commit the transaction after successful publishing
	if err := svc.DB.CommitTransaction(tx); err != nil {
		WriteResponse(w, http.StatusInternalServerError, nil)
		return
	}

	// Add location header
	var location = fmt.Sprintf("%s/%s", r.URL.Path, messagePayload.ID)

	// Send a success response after creating the workspace and publishing the event
	WriteResponse(w, http.StatusCreated, messagePayload, location)

}
