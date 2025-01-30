package services

import (
	"encoding/json"
	"net/http"
	"regexp"
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

// writeErrResponse sends an error response as JSON with a specified status code.
func writeErrResponse(w http.ResponseWriter, statusCode int, response interface{}) {

	w.Header().Set("Content-Type", "application/json")

	// We don't want to cache API responses so the client receives most curent data
	w.Header().Set("Cache-Control", "max-age=0")

	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
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
