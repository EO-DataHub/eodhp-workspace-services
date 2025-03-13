package services

import (
	"net/http"

	"github.com/EO-DataHub/eodhp-workspace-services/api/middleware"
	"github.com/EO-DataHub/eodhp-workspace-services/internal/authn"
	"github.com/gorilla/mux"
	"github.com/rs/zerolog"
)

// GetUsersService retrieves all users associated with a workspace.
func GetUsersService(svc *Service, w http.ResponseWriter, r *http.Request) {

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
	authorized, err := isUserWorkspaceAuthorized(svc.DB, claims, workspaceID, false)
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
	group, err := svc.KC.GetGroup(workspace.MemberGroup)

	if err != nil {
		logger.Error().Err(err).Str("member_group", workspace.MemberGroup).Msg("Failed to retrieve Keycloak group")
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

	logger.Info().Int("user_count", len(members)).Msg("Successfully retrieved workspace users")
	WriteResponse(w, http.StatusOK, members)
}

// GetUsersService retrieves all users associated with a workspace.
func GetUserService(svc *Service, w http.ResponseWriter, r *http.Request) {

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
	authorized, err := isUserWorkspaceAuthorized(svc.DB, claims, workspaceID, false)
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
	group, err := svc.KC.GetGroup(workspace.MemberGroup)

	if err != nil {
		logger.Error().Err(err).Str("member_group", workspace.MemberGroup).Msg("Failed to retrieve Keycloak group")
		WriteResponse(w, http.StatusInternalServerError, nil)
		return
	}

	userID, err := svc.KC.GetUserID(username)
	if err != nil {
		logger.Warn().Err(err).Str("username", username).Msg("User ID not found")
		WriteResponse(w, http.StatusNotFound, err.Error())
		return
	}

	// Get the members of the group
	member, err := svc.KC.GetGroupMember(group.ID, userID)

	if err != nil {
		logger.Error().Err(err).Str("group_id", group.ID).Str("user_id", userID).Msg("Failed to retrieve user membership")
		WriteResponse(w, http.StatusInternalServerError, nil)
		return
	}

	logger.Info().Str("user_id", userID).Msg("Successfully retrieved workspace user")
	WriteResponse(w, http.StatusOK, member)
}

// AddUserService adds a user to a workspace.
func AddUserService(svc *Service, w http.ResponseWriter, r *http.Request) {

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

	// Only account owners can remove users from a workspace
	authorized, err := isUserWorkspaceAuthorized(svc.DB, claims, workspaceID, true)
	if err != nil {
		logger.Error().Err(err).Str("workspace_id", workspaceID).Msg("Failed to authorize workspace")
		WriteResponse(w, http.StatusInternalServerError, nil)
		return
	}

	if !authorized {
		WriteResponse(w, http.StatusForbidden, nil)
		return
	}

	// Get the workspace member_group
	workspace, err := svc.DB.GetWorkspace(workspaceID)

	if err != nil {
		logger.Error().Err(err).Msg("Database error retrieving workspace")
		WriteResponse(w, http.StatusInternalServerError, nil)
		return
	}

	// Find the group ID from keycloak
	group, err := svc.KC.GetGroup(workspace.MemberGroup)

	if err != nil {
		logger.Error().Err(err).Str("member_group", workspace.MemberGroup).Msg("Failed to retrieve Keycloak group")
		WriteResponse(w, http.StatusInternalServerError, nil)
		return
	}

	userID, err := svc.KC.GetUserID(username)
	if err != nil {
		logger.Warn().Err(err).Str("username", username).Msg("User ID not found")
		WriteResponse(w, http.StatusNotFound, err.Error())
		return
	}

	// Add the user to the group in Keycloak
	err = svc.KC.AddMemberToGroup(userID, group.ID)
	if err != nil {
		logger.Error().Err(err).Str("user_id", userID).Str("group_id", group.ID).Msg("Failed to add user to group")
		WriteResponse(w, http.StatusInternalServerError, nil)
		return
	}

	logger.Info().Str("username", username).Str("group", group.Name).Msg("User added to workspace group successfully")
	WriteResponse(w, http.StatusNoContent, nil)
}

// RemoveUserService removes a user from a workspace.
func RemoveUserService(svc *Service, w http.ResponseWriter, r *http.Request) {

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

	// Only account owners can remove users from a workspace
	authorized, err := isUserWorkspaceAuthorized(svc.DB, claims, workspaceID, true)
	if err != nil {
		logger.Error().Err(err).Str("workspace_id", workspaceID).Msg("Failed to authorize workspace")
		WriteResponse(w, http.StatusInternalServerError, nil)
		return
	}

	if !authorized {
		WriteResponse(w, http.StatusForbidden, nil)
		return
	}

	// Get the workspace member_group
	workspace, err := svc.DB.GetWorkspace(workspaceID)

	if err != nil {
		logger.Error().Err(err).Msg("Database error retrieving workspace")
		WriteResponse(w, http.StatusInternalServerError, nil)
		return
	}

	// Find the group ID from keycloak
	group, err := svc.KC.GetGroup(workspace.MemberGroup)

	if err != nil {
		logger.Error().Err(err).Str("member_group", workspace.MemberGroup).Msg("Failed to retrieve Keycloak group")
		WriteResponse(w, http.StatusInternalServerError, nil)
		return
	}

	userID, err := svc.KC.GetUserID(username)
	if err != nil {
		logger.Warn().Err(err).Str("username", username).Msg("User ID not found")
		WriteResponse(w, http.StatusNotFound, err.Error())
		return
	}

	// Remove the user from the group in Keycloak
	err = svc.KC.RemoveMemberFromGroup(userID, group.ID)

	if err != nil {
		logger.Error().Err(err).Str("user_id", userID).Str("group_id", group.ID).Msg("Failed to remove user from group")
		WriteResponse(w, http.StatusInternalServerError, nil)
		return
	}

	logger.Info().Str("username", username).Str("group", group.Name).Msg("User removed from workspace group successfully")
	WriteResponse(w, http.StatusNoContent, nil)
}
