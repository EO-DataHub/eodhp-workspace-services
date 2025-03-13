package services

import (
	"encoding/json"
	"net/http"
	"regexp"

	"github.com/EO-DataHub/eodhp-workspace-services/db"
	"github.com/EO-DataHub/eodhp-workspace-services/internal/authn"
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
			return // **Return immediately to avoid multiple WriteHeader calls**
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

// isUserWorkspaceAuthorized checks if a user is authorized to access information in a workspace
func isUserWorkspaceAuthorized(db *db.WorkspaceDB, claims authn.Claims, workspace string, mustBeAccountOwner bool) (bool, error) {

	// Check if the user is an account owner
	if mustBeAccountOwner {
		if isMemberGroupAuthorized(workspace, claims.MemberGroups) {

			// Do they own the workspace
			isAccountOwner, err := db.IsUserAccountOwner(claims.Username, workspace)

			// Check for errors
			if err != nil {
				return false, err
			}

			// Return true if the user is the account owner
			if isAccountOwner {
				return true, nil
			}

			// Return false if the user is not the account owner
			return false, nil
		}
	}

	// If the user is not an account owner, check if they are a member of the workspace
	if isMemberGroupAuthorized(workspace, claims.MemberGroups) {
		return true, nil
	}

	// Return false if the user is not a member of the workspace or an account owner
	return false, nil
}
