package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"

	"github.com/EO-DataHub/eodhp-workspace-services/db"
	"github.com/EO-DataHub/eodhp-workspace-services/internal/authn"
	"github.com/rs/zerolog"
)

var dnsNameRegex = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$`)

func WriteResponse(w http.ResponseWriter, statusCode int, response interface{}, location ...string) {

	w.Header().Set("Content-Type", "application/json")

	// We don't want to cache API responses so the client receives most curent data
	w.Header().Set("Cache-Control", "max-age=0")

	// Conditionally set the Location header if provided
	if len(location) > 0 && location[0] != "" {
		w.Header().Set("Location", location[0])
	}

	w.WriteHeader(statusCode)

	if response != nil {
		if err := json.NewEncoder(w).Encode(response); err != nil {
			http.Error(w, "Failed to encode response", http.StatusInternalServerError)
			return
		}
	}
}

// HasRole checks if a user has a specific role in the JWT claims.
func HasRole(roles []string, role string) bool {
	for _, userRole := range roles {
		if userRole == role {
			return true
		}
	}
	return false
}

// Helper function to check if a member group is in the claims array
func isMemberGroupAuthorized(workspaceGroup string, claimsGroups []string) bool {
	for _, group := range claimsGroups {
		if workspaceGroup == group {
			return true
		}
	}
	return false
}

// isDNSCompatible returns true if the provided name is DNS-compatible
func IsDNSCompatible(name string) bool {
	return dnsNameRegex.MatchString(name)
}

// workspaceMembership is who the caller is in a workspace, before any admin check.
type workspaceMembership struct {
	username string
	member   bool
	// superuser is true for hub_admin and the workspaces service account, who can do anything
	superuser bool
}

// getWorkspaceMembership checks whether the caller is a member of the workspace. It does not
// check admin status, because that needs extra DB lookups - see isUserWorkspaceAdmin.
func getWorkspaceMembership(kc KeycloakClientInterface, claims authn.Claims, workspace string) (workspaceMembership, error) {

	membership := workspaceMembership{username: claims.Username}

	// hub_admin role is a superuser role
	if HasRole(claims.RealmAccess.Roles, "hub_admin") || claims.Username == "service-account-eodh-workspaces" {
		membership.member = true
		membership.superuser = true
		return membership, nil
	}

	// Get the groups from keycloak associated with the user
	memberGroups, err := kc.GetUserGroups(claims.Subject)
	if err != nil {
		return membership, err
	}

	membership.member = isMemberGroupAuthorized(workspace, memberGroups)
	return membership, nil
}

// isUserWorkspaceAdmin checks if a workspace member is the account owner or a workspace admin.
// It does not check membership or superuser roles - see getWorkspaceMembership.
func isUserWorkspaceAdmin(db db.WorkspaceDBInterface, username, workspace string) (bool, error) {

	// The account owner is an implicit admin on every workspace they own
	isAccountOwner, err := db.IsUserAccountOwner(username, workspace)
	if err != nil {
		return false, err
	}

	if isAccountOwner {
		return true, nil
	}

	// Otherwise, they must have been explicitly granted admin status on this workspace
	return db.IsUserWorkspaceAdmin(username, workspace)
}

// isUserWorkspaceAuthorized checks if a user is authorized to access information in a workspace
func isUserWorkspaceAuthorized(db db.WorkspaceDBInterface, kc KeycloakClientInterface, claims authn.Claims, workspace string, mustBeWorkspaceAdmin bool) (bool, error) {

	membership, err := getWorkspaceMembership(kc, claims, workspace)
	if err != nil || !membership.member {
		return false, err
	}

	if !mustBeWorkspaceAdmin || membership.superuser {
		return true, nil
	}

	return isUserWorkspaceAdmin(db, claims.Username, workspace)
}

// rejectAccountOwner checks whether username is the account owner for the workspace, and if so
// writes a 403 response with forbiddenMessage and returns true so the caller can stop handling
// the request. It also handles the 500 response on DB error. Returns false if the request should
// proceed.
func rejectAccountOwner(db db.WorkspaceDBInterface, logger *zerolog.Logger, w http.ResponseWriter, username, workspaceID, forbiddenMessage string) bool {

	isAccountOwner, err := db.IsUserAccountOwner(username, workspaceID)
	if err != nil {
		logger.Error().Err(err).Str("username", username).Str("workspace_id", workspaceID).Msg("Failed to check if user is account owner")
		WriteResponse(w, http.StatusInternalServerError, nil)
		return true
	}

	if isAccountOwner {
		logger.Warn().Str("username", username).Str("workspace_id", workspaceID).Msg(forbiddenMessage)
		WriteResponse(w, http.StatusForbidden, forbiddenMessage)
		return true
	}

	return false
}

func makeHTTPRequest(method, url string, headers map[string]string, body []byte) ([]byte, error) {
	req, err := http.NewRequest(method, url, bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}

	for key, value := range headers {
		req.Header.Set(key, value)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed request, status: %s", resp.Status)
	}

	return io.ReadAll(resp.Body)
}
