package services

import (
	"net/http"

	"github.com/EO-DataHub/eodhp-workspace-services/api/middleware"
	"github.com/EO-DataHub/eodhp-workspace-services/internal/authn"
	"github.com/gorilla/mux"
	"github.com/rs/zerolog"
)

// GetUsersService retrieves all users associated with a workspace.
func (svc *WorkspaceService) GetUsersService(w http.ResponseWriter, r *http.Request) {

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

	// Get information about the workspace
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

	// Get the members of the group
	members, err := svc.KC.GetGroupMembers(group.ID)

	if err != nil {
		logger.Error().Err(err).Str("group_id", group.ID).Msg("Failed to retrieve group members")
		WriteResponse(w, http.StatusInternalServerError, nil)
		return
	}

	WriteResponse(w, http.StatusOK, members)
}

// GetUsersService retrieves all users associated with a workspace.
func (svc *WorkspaceService) GetUserService(w http.ResponseWriter, r *http.Request) {

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
	username := mux.Vars(r)["username"]

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

	// Get information about the workspace
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

	// Get the members of the group
	member, err := svc.KC.GetGroupMember(group.ID, user.ID)

	if err != nil {
		logger.Error().Err(err).Str("group_id", group.ID).Str("user_id", user.ID).Msg("Failed to retrieve user membership")
		WriteResponse(w, http.StatusInternalServerError, nil)
		return
	}

	WriteResponse(w, http.StatusOK, member)
}

// AddUserService adds a user to a workspace.
func (svc *WorkspaceService) AddUserService(w http.ResponseWriter, r *http.Request) {

	logger := zerolog.Ctx(r.Context())

	// Extract claims from the request context to identify the user
	claims, ok := r.Context().Value(middleware.ClaimsKey).(authn.Claims)
	if !ok {
		logger.Warn().Msg("Unauthorized request: missing claims")
		WriteResponse(w, http.StatusUnauthorized, nil)
		return
	}

	// Parse the workspace ID and user ID from the URL path
	workspaceID := mux.Vars(r)["workspace-id"]
	username := mux.Vars(r)["username"]

	// Only the account owner or a workspace admin can add users to a workspace
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

	// Add the user to the group in Keycloak
	err = svc.KC.AddMemberToGroup(user.ID, group.ID)
	if err != nil {
		logger.Error().Err(err).Str("user_id", user.ID).Str("group_id", group.ID).Msg("Failed to add user to group")
		WriteResponse(w, http.StatusInternalServerError, nil)
		return
	}

	logger.Info().Str("username", username).Str("group", group.Name).Msg("User added to workspace group successfully")
	WriteResponse(w, http.StatusNoContent, nil)
}

// RemoveUserService removes a user from a workspace.
func (svc *WorkspaceService) RemoveUserService(w http.ResponseWriter, r *http.Request) {

	logger := zerolog.Ctx(r.Context())

	// Extract claims from the request context to identify the user
	claims, ok := r.Context().Value(middleware.ClaimsKey).(authn.Claims)
	if !ok {
		logger.Warn().Msg("Unauthorized request: missing claims")
		WriteResponse(w, http.StatusUnauthorized, nil)
		return
	}

	// Parse the workspace ID and user ID from the URL path
	workspaceID := mux.Vars(r)["workspace-id"]
	username := mux.Vars(r)["username"]

	// Only the account owner or a workspace admin can remove users from a workspace
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

	// Account owners cannot remove themselves from a group
	isAccountOwner, err := svc.DB.IsUserAccountOwner(username, workspaceID)

	if err != nil {
		logger.Error().Err(err).Str("username", username).Str("workspace_id", workspaceID).Msg("Failed to check if user is account owner")
		WriteResponse(w, http.StatusInternalServerError, nil)
		return
	}

	if isAccountOwner {
		logger.Warn().Str("username", username).Str("workspace_id", workspaceID).Msg("Account owners cannot remove themselves from a workspace")
		WriteResponse(w, http.StatusForbidden, "Account owners cannot remove themselves from a workspace")
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

	// Remove the user from the group in Keycloak first - if this fails, no DB change has
	// been made yet, so the request can be safely retried without leaving the user in a
	// half-removed state (still a member but silently stripped of admin status).
	err = svc.KC.RemoveMemberFromGroup(user.ID, group.ID)

	if err != nil {
		logger.Error().Err(err).Str("user_id", user.ID).Str("group_id", group.ID).Msg("Failed to remove user from group")
		WriteResponse(w, http.StatusInternalServerError, nil)
		return
	}

	// A user's admin status can't outlive their workspace membership, so revoke it now
	// that they've been removed from the group.
	if err := svc.DB.RemoveWorkspaceAdmin(username, workspaceID); err != nil {
		logger.Error().Err(err).Str("username", username).Str("workspace_id", workspaceID).Msg("Failed to remove workspace admin status")
		WriteResponse(w, http.StatusInternalServerError, nil)
		return
	}

	logger.Info().Str("username", username).Str("group", group.Name).Msg("User removed from workspace group successfully")
	WriteResponse(w, http.StatusNoContent, nil)
}
