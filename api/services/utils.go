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

// isUserWorkspaceAuthorized checks if a user is authorized to access information in a workspace
func isUserWorkspaceAuthorized(db db.WorkspaceDBInterface, kc KeycloakClientInterface, claims authn.Claims, workspace string, mustBeAccountOwner bool) (bool, error) {

	// hub_admin role is a superuser role
	if HasRole(claims.RealmAccess.Roles, "hub_admin") {
		return true, nil
	}

	if claims.Username == "service-account-eodh-workspaces" {
		return true, nil
	}

	// Get the groups from keycloak associated with the user
	memberGroups, err := kc.GetUserGroups(claims.Subject)
	if err != nil {
		return false, err
	}

	// Check if the user is an account owner
	if mustBeAccountOwner {
		if isMemberGroupAuthorized(workspace, memberGroups) {

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
	if isMemberGroupAuthorized(workspace, memberGroups) {
		return true, nil
	}

	// Return false if the user is not a member of the workspace or an account owner
	return false, nil
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
