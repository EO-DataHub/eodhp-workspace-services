package services

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/rs/zerolog/log"
)

// GetUsersService retrieves all users associated with a workspace.
func GetUsersService(svc *Service, w http.ResponseWriter, r *http.Request) {

	// Parse the workspace ID from the URL path
	workspaceID := mux.Vars(r)["workspace-id"]

	// Get information about the workspace
	workspace, err := svc.DB.GetWorkspace(workspaceID)

	if err != nil {
		WriteResponse(w, http.StatusInternalServerError, nil)
		return
	}

	// Find the group ID from keycloak
	group, err := svc.KC.GetGroup(workspace.MemberGroup)

	if err != nil {
		WriteResponse(w, http.StatusInternalServerError, nil)
		return
	}

	// Get the members of the group
	members, err := svc.KC.GetGroupMembers(group.ID)

	if err != nil {
		WriteResponse(w, http.StatusInternalServerError, nil)
		return
	}

	// Return the members
	WriteResponse(w, http.StatusOK, members)
}

// GetUsersService retrieves all users associated with a workspace.
func GetUserService(svc *Service, w http.ResponseWriter, r *http.Request) {

	// Parse the workspace ID from the URL path
	workspaceID := mux.Vars(r)["workspace-id"]
	username := mux.Vars(r)["username"]

	// Get information about the workspace
	workspace, err := svc.DB.GetWorkspace(workspaceID)

	if err != nil {
		WriteResponse(w, http.StatusInternalServerError, nil)
		return
	}

	// Find the group ID from keycloak
	group, err := svc.KC.GetGroup(workspace.MemberGroup)

	if err != nil {
		WriteResponse(w, http.StatusInternalServerError, nil)
		return
	}

	userID, err := svc.KC.GetUserID(username)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get user ID")
		WriteResponse(w, http.StatusInternalServerError, nil)
		return
	}

	// Get the members of the group
	member, err := svc.KC.GetGroupMember(group.ID, userID)

	if err != nil {
		WriteResponse(w, http.StatusInternalServerError, nil)
		return
	}

	// Return the member
	WriteResponse(w, http.StatusOK, member)
}

// AddUserService adds a user to a workspace.
func AddUserService(svc *Service, w http.ResponseWriter, r *http.Request) {

	// Parse the workspace ID and user ID from the URL path
	workspaceID := mux.Vars(r)["workspace-id"]
	username := mux.Vars(r)["username"]

	// Get the workspace member_group
	workspace, err := svc.DB.GetWorkspace(workspaceID)

	if err != nil {
		WriteResponse(w, http.StatusInternalServerError, nil)
		return
	}

	// Find the group ID from keycloak
	group, err := svc.KC.GetGroup(workspace.MemberGroup)

	if err != nil {
		WriteResponse(w, http.StatusInternalServerError, nil)
		return
	}

	userID, err := svc.KC.GetUserID(username)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get user ID")
		WriteResponse(w, http.StatusInternalServerError, nil)
		return
	}

	// Add the user to the group in Keycloak
	err = svc.KC.AddMemberToGroup(userID, group.ID)
	if err != nil {
		log.Error().Err(err).Msg("Failed to add member to group")
		WriteResponse(w, http.StatusInternalServerError, nil)
		return
	}

	WriteResponse(w, http.StatusNoContent, nil)
}

// RemoveUserService removes a user from a workspace.
func RemoveUserService(svc *Service, w http.ResponseWriter, r *http.Request) {

	// Parse the workspace ID and user ID from the URL path
	workspaceID := mux.Vars(r)["workspace-id"]
	username := mux.Vars(r)["username"]

	// Get the workspace member_group
	workspace, err := svc.DB.GetWorkspace(workspaceID)

	if err != nil {
		WriteResponse(w, http.StatusInternalServerError, nil)
		return
	}

	// Find the group ID from keycloak
	group, err := svc.KC.GetGroup(workspace.MemberGroup)

	if err != nil {
		WriteResponse(w, http.StatusInternalServerError, nil)
		return
	}

	userID, err := svc.KC.GetUserID(username)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get user ID")
		WriteResponse(w, http.StatusInternalServerError, nil)
		return
	}

	// Remove the user from the group in Keycloak
	err = svc.KC.RemoveMemberFromGroup(userID, group.ID)

	if err != nil {
		log.Error().Err(err).Msg("Failed to remove member from group")
		WriteResponse(w, http.StatusInternalServerError, nil)
		return
	}

	WriteResponse(w, http.StatusNoContent, nil)
}
