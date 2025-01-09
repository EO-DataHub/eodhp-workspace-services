package services

import (
	"encoding/json"
	"net/http"

	"github.com/EO-DataHub/eodhp-workspace-services/db"
	"github.com/EO-DataHub/eodhp-workspace-services/models"
	"github.com/gorilla/mux"
	"github.com/rs/zerolog/log"
)

// GetUsersService retrieves all users associated with a workspace.
func GetUsersService(workspaceDB *db.WorkspaceDB, kc *KeycloakClient, w http.ResponseWriter, r *http.Request) {

	// Parse the workspace ID from the URL path
	workspaceID := mux.Vars(r)["workspace-id"]

	// Get information about the workspace
	workspace, err := workspaceDB.GetWorkspace(workspaceID)

	if err != nil {
		HandleErrResponse(w, http.StatusInternalServerError, err)
		return
	}

	// Find the group ID from keycloak
	group, err := kc.GetGroup(workspace.MemberGroup)

	if err != nil {
		HandleErrResponse(w, http.StatusInternalServerError, err)
		return
	}

	// Get the members of the group
	members, err := kc.GetGroupMembers(group.ID)

	if err != nil {
		HandleErrResponse(w, http.StatusInternalServerError, err)
		return
	}

	HandleSuccessResponse(w, http.StatusOK, nil, members, "")
}

// GetUsersService retrieves all users associated with a workspace.
func GetUserService(workspaceDB *db.WorkspaceDB, kc *KeycloakClient, w http.ResponseWriter, r *http.Request) {

	// Parse the workspace ID from the URL path
	workspaceID := mux.Vars(r)["workspace-id"]
	userID := mux.Vars(r)["user-id"]
	// Get information about the workspace
	workspace, err := workspaceDB.GetWorkspace(workspaceID)

	if err != nil {
		HandleErrResponse(w, http.StatusInternalServerError, err)
		return
	}

	// Find the group ID from keycloak
	group, err := kc.GetGroup(workspace.MemberGroup)

	if err != nil {
		HandleErrResponse(w, http.StatusInternalServerError, err)
		return
	}

	// Get the members of the group
	member, err := kc.GetGroupMember(group.ID, userID)

	if err != nil {
		HandleErrResponse(w, http.StatusInternalServerError, err)
		return
	}

	HandleSuccessResponse(w, http.StatusOK, nil, member, "")
}

// GetUsersService retrieves all users associated with a workspace.
func AddUserService(workspaceDB *db.WorkspaceDB, kc *KeycloakClient, w http.ResponseWriter, r *http.Request) {

	// Decode the request body into a UserMembershipRequest struct
	var messagePayload models.UserMembershipRequest
	if err := json.NewDecoder(r.Body).Decode(&messagePayload); err != nil {
		log.Error().Err(err).Msg("Invalid request payload")
		HandleErrResponse(w, http.StatusInternalServerError, err)
		return
	}

	// Validate required fields in UserMembershipRequest
	if messagePayload.UserID == "" || messagePayload.GroupID == "" {
		http.Error(w, "userID and groupID are required fields", http.StatusBadRequest)
		return
	}

	// Add the user to the group in Keycloak
	err := kc.AddMemberToGroup(messagePayload.UserID, messagePayload.GroupID)
	if err != nil {
		log.Error().Err(err).Msg("Failed to add member to group")
		HandleErrResponse(w, http.StatusInternalServerError, err)
		return
	}

	// Set the status code to 204 and return without a body
	w.WriteHeader(http.StatusNoContent)
}

// GetUsersService retrieves all users associated with a workspace.
func RemoveUserService(workspaceDB *db.WorkspaceDB, kc *KeycloakClient, w http.ResponseWriter, r *http.Request) {

	// Decode the request body into a UserMembershipRequest struct
	var messagePayload models.UserMembershipRequest
	if err := json.NewDecoder(r.Body).Decode(&messagePayload); err != nil {
		log.Error().Err(err).Msg("Invalid request payload")
		HandleErrResponse(w, http.StatusInternalServerError, err)
		return
	}

	// Validate required fields in UserMembershipRequest
	if messagePayload.UserID == "" || messagePayload.GroupID == "" {
		http.Error(w, "userID and groupID are required fields", http.StatusBadRequest)
		return
	}

	// Remove the user from the group in Keycloak
	err := kc.RemoveMemberFromGroup(messagePayload.UserID, messagePayload.GroupID)
	if err != nil {
		log.Error().Err(err).Msg("Failed to remove member from group")
		HandleErrResponse(w, http.StatusInternalServerError, err)
		return
	}

	// Set the status code to 204 and return without a body
	w.WriteHeader(http.StatusNoContent)
}
