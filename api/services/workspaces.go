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

	// Ensure workspaces is not nil, return an empty slice if no workspaces are found
	if workspaces == nil {
		workspaces = []ws_manager.WorkspaceSettings{}
	}

	// Send a success response with the retrieved workspaces data
	WriteResponse(w, http.StatusOK, workspaces)

}

// GetWorkspaceService retrieves an individual workspace accessible to the authenticated user's groups.
func GetWorkspaceService(svc *Service, w http.ResponseWriter, r *http.Request) {

	// Extract groups the user is a member of from the claims
	claims, ok := r.Context().Value(middleware.ClaimsKey).(authn.Claims)
	if !ok {
		WriteResponse(w, http.StatusUnauthorized, nil)
		return
	}

	// Parse the workspace ID from the URL path
	workspaceID := mux.Vars(r)["workspace-id"]

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

	// Decode the request body into a Workspace struct
	var wsSettings ws_manager.WorkspaceSettings
	if err := json.NewDecoder(r.Body).Decode(&wsSettings); err != nil {
		log.Error().Err(err).Msg("Invalid request payload")
		WriteResponse(w, http.StatusBadRequest, nil)
		return
	}

	// Check the name is DNS-compatible
	if !IsDNSCompatible(wsSettings.Name) {
		WriteResponse(w, http.StatusBadRequest, fmt.Errorf("invalid workspace name: must contain only a-z and -, not start with - and be less than 63 characters"))
		return
	}

	// Check that the workspace name does not already exist
	workspaceExists, err := svc.DB.CheckWorkspaceExists(wsSettings.Name)
	if err != nil {
		WriteResponse(w, http.StatusInternalServerError, nil)
		return
	}

	// Return a conflict response if the workspace name already exists
	if workspaceExists {
		log.Error().Msgf("workspace with name %s already exists", wsSettings.Name)
		WriteResponse(w, http.StatusConflict, fmt.Errorf("workspace with name %s already exists", wsSettings.Name))
		return
	}

	// Check that the account exists and the user is the account owner
	account, err := svc.DB.CheckAccountExists(wsSettings.Account)
	if err != nil {
		WriteResponse(w, http.StatusInternalServerError, nil)
		return
	}

	// Return a not found response if the account does not exist
	if !account {
		log.Error().Msgf("account with ID %s not found", wsSettings.Account)
		WriteResponse(w, http.StatusNotFound, fmt.Errorf("The account associated with this workspace does not exist"))
		return
	}

	// Create a group in Keycloak - the group name is the same as the workspace name
	wsSettings.MemberGroup = wsSettings.Name
	statusCode, err := svc.KC.CreateGroup(wsSettings.MemberGroup)

	if err != nil {
		WriteResponse(w, statusCode, nil)
		return
	}

	log.Info().Msgf("Group %s created successfully", wsSettings.MemberGroup)

	// Find the group ID just created from keycloak
	group, err := svc.KC.GetGroup(wsSettings.MemberGroup)

	if err != nil {
		WriteResponse(w, http.StatusInternalServerError, nil)
		return
	}

	accountOwnerID := claims.Subject
	err = svc.KC.AddMemberToGroup(accountOwnerID, group.ID)
	if err != nil {
		log.Error().Err(err).Msg("Failed to add member to group")
		WriteResponse(w, http.StatusInternalServerError, nil)
		return
	}

	// Begin the workspace creation transaction
	wsSettings.Status = "creating"

	// Define default object and block stores
	wsSettings.Stores = &[]ws_manager.Stores{
		{
			Object: []ws_manager.ObjectStore{
				{Name: wsSettings.Name},
			},
			Block: []ws_manager.BlockStore{
				{Name: wsSettings.Name},
			},
		},
	}
	tx, err := svc.DB.CreateWorkspace(&wsSettings)
	if err != nil {
		WriteResponse(w, http.StatusInternalServerError, nil)
		return
	}

	// Publish a message for workspace creation
	err = svc.Publisher.Publish(wsSettings)
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
	var location = fmt.Sprintf("%s/%s", r.URL.Path, wsSettings.ID)

	// Send a success response after creating the workspace and publishing the event
	WriteResponse(w, http.StatusCreated, wsSettings, location)

}
