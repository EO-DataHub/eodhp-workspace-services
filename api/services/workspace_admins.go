package services

import (
	"net/http"

	"github.com/EO-DataHub/eodhp-workspace-services/api/middleware"
	"github.com/EO-DataHub/eodhp-workspace-services/internal/authn"
	"github.com/gorilla/mux"
	"github.com/rs/zerolog"
)

// GetWorkspaceAdminsService retrieves the usernames of every explicitly-granted admin on a workspace.
// It does not include the account owner, who is an implicit admin on every workspace they own.
func (svc *WorkspaceService) GetWorkspaceAdminsService(w http.ResponseWriter, r *http.Request) {

	logger := zerolog.Ctx(r.Context())

	// Extract claims from the request context to identify the user
	claims, ok := r.Context().Value(middleware.ClaimsKey).(authn.Claims)
	if !ok {
		logger.Warn().Msg("Unauthorized request: missing claims")
		WriteResponse(w, http.StatusUnauthorized, nil)
		return
	}

	// Parse the workspace ID from the URL path
	workspaceID := mux.Vars(r)["workspace-id"]

	// Check if the user can access the workspace
	authorized, err := isUserWorkspaceAuthorized(svc.DB, svc.KC, claims, workspaceID, false)
	if err != nil {
		logger.Error().Err(err).Str("workspace_id", workspaceID).Msg("Failed to authorize workspace")
		WriteResponse(w, http.StatusInternalServerError, nil)
		return
	}

	if !authorized {
		WriteResponse(w, http.StatusForbidden, nil)
		return
	}

	admins, err := svc.DB.GetWorkspaceAdmins(workspaceID)
	if err != nil {
		logger.Error().Err(err).Str("workspace_id", workspaceID).Msg("Database error retrieving workspace admins")
		WriteResponse(w, http.StatusInternalServerError, nil)
		return
	}

	WriteResponse(w, http.StatusOK, admins)
}

// AddWorkspaceAdminService grants a workspace member admin status.
func (svc *WorkspaceService) AddWorkspaceAdminService(w http.ResponseWriter, r *http.Request) {

	logger := zerolog.Ctx(r.Context())

	// Extract claims from the request context to identify the user
	claims, ok := r.Context().Value(middleware.ClaimsKey).(authn.Claims)
	if !ok {
		logger.Warn().Msg("Unauthorized request: missing claims")
		WriteResponse(w, http.StatusUnauthorized, nil)
		return
	}

	// Parse the workspace ID and username from the URL path
	workspaceID := mux.Vars(r)["workspace-id"]
	username := mux.Vars(r)["username"]

	// Only the account owner or a workspace admin can grant admin status
	authorized, err := isUserWorkspaceAuthorized(svc.DB, svc.KC, claims, workspaceID, true)
	if err != nil {
		logger.Error().Err(err).Str("workspace_id", workspaceID).Msg("Failed to authorize workspace")
		WriteResponse(w, http.StatusInternalServerError, nil)
		return
	}

	if !authorized {
		WriteResponse(w, http.StatusForbidden, nil)
		return
	}

	workspace, err := svc.DB.GetWorkspace(workspaceID)
	if err != nil {
		logger.Error().Err(err).Msg("Database error retrieving workspace")
		WriteResponse(w, http.StatusInternalServerError, nil)
		return
	}

	// Find the group ID from keycloak
	group, err := svc.KC.GetGroup(workspace.Name)
	if err != nil {
		logger.Error().Err(err).Str("name", workspace.Name).Msg("Failed to retrieve Keycloak group")
		WriteResponse(w, http.StatusInternalServerError, nil)
		return
	}

	user, err := svc.KC.GetUser(username)
	if err != nil {
		logger.Warn().Err(err).Str("username", username).Msg("User ID not found")
		WriteResponse(w, http.StatusNotFound, err.Error())
		return
	}

	// Admin status can only be granted to an existing workspace member
	members, err := svc.KC.GetGroupMembers(group.ID)
	if err != nil {
		logger.Error().Err(err).Str("group_id", group.ID).Msg("Failed to retrieve group members")
		WriteResponse(w, http.StatusInternalServerError, nil)
		return
	}

	isMember := false
	for _, member := range members {
		if member.ID == user.ID {
			isMember = true
			break
		}
	}

	if !isMember {
		logger.Warn().Str("username", username).Str("workspace_id", workspaceID).Msg("User is not a member of the workspace")
		WriteResponse(w, http.StatusBadRequest, "User must be a member of the workspace before being granted admin status")
		return
	}

	if err := svc.DB.AddWorkspaceAdmin(username, workspaceID, claims.Username); err != nil {
		logger.Error().Err(err).Str("username", username).Str("workspace_id", workspaceID).Msg("Failed to add workspace admin")
		WriteResponse(w, http.StatusInternalServerError, nil)
		return
	}

	logger.Info().Str("username", username).Str("workspace_id", workspaceID).Msg("Workspace admin granted successfully")
	WriteResponse(w, http.StatusNoContent, nil)
}

// RemoveWorkspaceAdminService revokes a user's admin status on a workspace.
func (svc *WorkspaceService) RemoveWorkspaceAdminService(w http.ResponseWriter, r *http.Request) {

	logger := zerolog.Ctx(r.Context())

	// Extract claims from the request context to identify the user
	claims, ok := r.Context().Value(middleware.ClaimsKey).(authn.Claims)
	if !ok {
		logger.Warn().Msg("Unauthorized request: missing claims")
		WriteResponse(w, http.StatusUnauthorized, nil)
		return
	}

	// Parse the workspace ID and username from the URL path
	workspaceID := mux.Vars(r)["workspace-id"]
	username := mux.Vars(r)["username"]

	// Only the account owner or a workspace admin can revoke admin status
	authorized, err := isUserWorkspaceAuthorized(svc.DB, svc.KC, claims, workspaceID, true)
	if err != nil {
		logger.Error().Err(err).Str("workspace_id", workspaceID).Msg("Failed to authorize workspace")
		WriteResponse(w, http.StatusInternalServerError, nil)
		return
	}

	if !authorized {
		WriteResponse(w, http.StatusForbidden, nil)
		return
	}

	if err := svc.DB.RemoveWorkspaceAdmin(username, workspaceID); err != nil {
		logger.Error().Err(err).Str("username", username).Str("workspace_id", workspaceID).Msg("Failed to remove workspace admin")
		WriteResponse(w, http.StatusInternalServerError, nil)
		return
	}

	logger.Info().Str("username", username).Str("workspace_id", workspaceID).Msg("Workspace admin revoked successfully")
	WriteResponse(w, http.StatusNoContent, nil)
}
