package services

import (
	"encoding/json"
	"fmt"

	"net/http"

	ws_manager "github.com/EO-DataHub/eodhp-workspace-manager/models"
	"github.com/EO-DataHub/eodhp-workspace-services/api/middleware"
	"github.com/EO-DataHub/eodhp-workspace-services/internal/authn"
	"github.com/gorilla/mux"
	"github.com/rs/zerolog"
)

// GetWorkspacesService retrieves all workspaces accessible to the authenticated user's groups.
func GetWorkspacesService(svc *Service, w http.ResponseWriter, r *http.Request) {

	logger := zerolog.Ctx(r.Context())

	// Extract groups the user is a member of from the claims
	claims, ok := r.Context().Value(middleware.ClaimsKey).(authn.Claims)
	if !ok {
		logger.Warn().Msg("Unauthorized request: missing claims")
		WriteResponse(w, http.StatusUnauthorized, nil)
		return
	}

	// Retrieve workspaces assigned to these groups
	workspaces, err := svc.DB.GetUserWorkspaces(claims.MemberGroups)
	if err != nil {
		logger.Error().Err(err).Msg("Database error retrieving workspaces")
		WriteResponse(w, http.StatusInternalServerError, nil)
		return
	}

	// If using a workspace scoped token, only return the workspace the token is scoped to
	if claims.Workspace != "" {
		var scopedWorkspaces []ws_manager.WorkspaceSettings
		for _, ws := range workspaces {
			if ws.Name == claims.Workspace {
				scopedWorkspaces = []ws_manager.WorkspaceSettings{ws}
				WriteResponse(w, http.StatusOK, scopedWorkspaces)
				return
			}
		}
		// If the workspace is not found, return unauthorized
		WriteResponse(w, http.StatusUnauthorized, nil)
		return
	}

	// Ensure workspaces is not nil, return an empty slice if no workspaces are found
	if workspaces == nil {
		workspaces = []ws_manager.WorkspaceSettings{}
	}

	WriteResponse(w, http.StatusOK, workspaces)

}

// GetWorkspaceService retrieves an individual workspace accessible to the authenticated user's groups.
func GetWorkspaceService(svc *Service, w http.ResponseWriter, r *http.Request) {

	logger := zerolog.Ctx(r.Context())

	// Extract groups the user is a member of from the claims
	claims, ok := r.Context().Value(middleware.ClaimsKey).(authn.Claims)
	if !ok {
		logger.Warn().Msg("Unauthorized request: missing claims")
		WriteResponse(w, http.StatusUnauthorized, nil)
		return
	}

	// Parse the workspace ID from the URL path
	workspaceID := mux.Vars(r)["workspace-id"]
	logger.Info().Str("workspace_id", workspaceID).Msg("Retrieving workspace")

	// Retrieve account associated with the user's username
	workspace, err := svc.DB.GetWorkspace(workspaceID)

	if err != nil {
		logger.Error().Err(err).Str("workspace_id", workspaceID).Msg("Database error retrieving workspace")
		WriteResponse(w, http.StatusNotFound, "Workspace does not exist.")
		return
	}

	// Check if the account owner matches any of the claims member groups
	if !isMemberGroupAuthorized(workspace.MemberGroup, claims.MemberGroups) {
		logger.Warn().Str("workspace_id", workspaceID).Str("user", claims.Username).Msg("Access denied: user not in authorized groups")
		WriteResponse(w, http.StatusForbidden, nil)
		return
	}

	// If using a workspace scoped token, only return the workspace the token is scoped to
	if claims.Workspace != "" {
		if claims.Workspace == workspace.Name {
			WriteResponse(w, http.StatusOK, workspace)
			return
		}

		// If the workspace is not found, return unauthorized
		WriteResponse(w, http.StatusUnauthorized, nil)
		return
	}

	WriteResponse(w, http.StatusOK, *workspace)
}

// CreateWorkspaceService handles creating a new workspace and publishing its creation event.
func CreateWorkspaceService(svc *Service, w http.ResponseWriter, r *http.Request) {

	logger := zerolog.Ctx(r.Context())

	// Extract the claims to get the users KC ID
	claims, ok := r.Context().Value(middleware.ClaimsKey).(authn.Claims)
	if !ok {
		logger.Warn().Msg("Unauthorized request: missing claims")
		WriteResponse(w, http.StatusUnauthorized, nil)
		return
	}

	// Workspace Scoped tokens not authorized to create a workspace
	if claims.Workspace != "" {
		logger.Warn().Msg("Unauthorized request: workspace scoped token")
		WriteResponse(w, http.StatusUnauthorized, nil)
		return
	}

	// Decode the request body into a Workspace struct
	var wsSettings ws_manager.WorkspaceSettings
	if err := json.NewDecoder(r.Body).Decode(&wsSettings); err != nil {
		logger.Warn().Err(err).Msg("Invalid request payload")
		WriteResponse(w, http.StatusBadRequest, nil)
		return
	}

	logger.Info().Str("workspace_name", wsSettings.Name).Msg("Validating workspace name")

	// Check the name is DNS-compatible
	if !IsDNSCompatible(wsSettings.Name) {
		logger.Warn().Str("workspace_name", wsSettings.Name).Msg("Invalid workspace name. Not DNS compatible")
		WriteResponse(w, http.StatusBadRequest, fmt.Errorf("invalid workspace name: must contain only a-z and -, not start with - and be less than 63 characters"))
		return
	}

	// Check that the workspace name does not already exist
	workspaceExists, err := svc.DB.CheckWorkspaceExists(wsSettings.Name)
	if err != nil {
		logger.Error().Err(err).Msg("Database error checking workspace existence")
		WriteResponse(w, http.StatusInternalServerError, nil)
		return
	}

	// Return a conflict response if the workspace name already exists
	if workspaceExists {
		logger.Warn().Str("workspace_name", wsSettings.Name).Msg("Workspace name already exists")
		WriteResponse(w, http.StatusConflict, fmt.Errorf("workspace with name %s already exists", wsSettings.Name))
		return
	}

	// Check that the account exists and the user is the account owner
	account, err := svc.DB.CheckAccountExists(wsSettings.Account)
	if err != nil {
		logger.Error().Err(err).Msg("Database error checking account existence")
		WriteResponse(w, http.StatusInternalServerError, nil)
		return
	}

	// Return a not found response if the account does not exist
	if !account {
		logger.Warn().Str("account_id", wsSettings.Account.String()).Msg("Account does not exist")
		WriteResponse(w, http.StatusNotFound, fmt.Errorf("The account associated with this workspace does not exist"))
		return
	}

	// Create a group in Keycloak - the group name is the same as the workspace name
	wsSettings.MemberGroup = wsSettings.Name
	statusCode, err := svc.KC.CreateGroup(wsSettings.MemberGroup)

	if err != nil {
		logger.Error().Err(err).Str("group_name", wsSettings.MemberGroup).Msg("Failed to create Keycloak group")
		WriteResponse(w, statusCode, nil)
		return
	}

	logger.Info().Str("group_name", wsSettings.MemberGroup).Msg("Group created successfully")

	// Find the group ID just created from keycloak
	group, err := svc.KC.GetGroup(wsSettings.MemberGroup)

	if err != nil {
		logger.Error().Err(err).Str("group_name", wsSettings.MemberGroup).Msg("Failed to retrieve Keycloak group")
		WriteResponse(w, http.StatusInternalServerError, nil)
		return
	}

	accountOwnerID := claims.Subject
	err = svc.KC.AddMemberToGroup(accountOwnerID, group.ID)
	if err != nil {
		logger.Error().Err(err).Str("user_id", claims.Subject).Str("group_id", group.ID).Msg("Failed to add user to group")
		WriteResponse(w, http.StatusInternalServerError, nil)
		return
	}

	logger.Info().Str("user_id", claims.Subject).Str("group_id", group.ID).Msg("User added to Keycloak group successfully")

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
		logger.Error().Err(err).Msg("Database error creating workspace")
		WriteResponse(w, http.StatusInternalServerError, nil)
		return
	}

	// Publish a message for workspace creation
	err = svc.Publisher.Publish(wsSettings)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to publish workspace creation event")
		tx.Rollback()
		WriteResponse(w, http.StatusInternalServerError, nil)
		return
	}

	// Commit the transaction after successful publishing
	if err := svc.DB.CommitTransaction(tx); err != nil {
		logger.Error().Err(err).Msg("Failed to commit transaction")
		WriteResponse(w, http.StatusInternalServerError, nil)
		return
	}

	logger.Info().Str("workspace_name", wsSettings.Name).Msg("Workspace created successfully")

	// Send response
	var location = fmt.Sprintf("%s/%s", r.URL.Path, wsSettings.ID)
	WriteResponse(w, http.StatusCreated, wsSettings, location)

}

// Deletes a workspace and its associated resources.
func DeleteWorkspaceService(svc *Service, w http.ResponseWriter, r *http.Request) {

	logger := zerolog.Ctx(r.Context())

	// Extract the claims to get the users KC ID
	claims, ok := r.Context().Value(middleware.ClaimsKey).(authn.Claims)
	if !ok {
		logger.Warn().Msg("Unauthorized request: missing claims")
		WriteResponse(w, http.StatusUnauthorized, nil)
		return
	}

	// Workspace Scoped tokens not authorized to delete workspace
	if claims.Workspace != "" {
		logger.Warn().Msg("Unauthorized request: workspace scoped token")
		WriteResponse(w, http.StatusUnauthorized, nil)
		return
	}

	// Parse the workspace ID from the URL path
	workspaceID := mux.Vars(r)["workspace-id"]

	var wsSettings ws_manager.WorkspaceSettings
	wsSettings.Name = workspaceID
	wsSettings.Status = "deleting"

	// Publish a message for workspace creation
	err := svc.Publisher.Publish(wsSettings)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to publish workspace deletion event")
		WriteResponse(w, http.StatusInternalServerError, nil)
		return
	}

	WriteResponse(w, http.StatusNoContent, nil)

}
