package services

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"

	ws_services "github.com/EO-DataHub/eodhp-workspace-services/models"
	"github.com/lib/pq"
	"github.com/rs/zerolog/log"
)

var dnsNameRegex = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$`)

// HandleErrResponse formats and sends error details, including pq.Error specifics.
func HandleErrResponse(w http.ResponseWriter, statusCode int, err error) {
	var pqErr *pq.Error
	var response ws_services.Response
	var syntaxErr *json.SyntaxError
	var unmarshalTypeErr *json.UnmarshalTypeError

	// Check if the error is a PostgreSQL-specific error (pq.Error)
	if errors.As(err, &pqErr) {

		// Log the error details but don't expose them to the client
		log.Error().
			Err(err).
			Msg(fmt.Sprintf("Database error: Code=%s, Message=%s", pqErr.Code.Name(), pqErr.Message))

		response = ws_services.Response{
			Success:      0,
			ErrorCode:    "internal_server_error",
			ErrorDetails: "An internal server error occurred. Please try again later.",
		}
		// Check if the error is a JSON syntax error
	} else if errors.As(err, &syntaxErr) {
		response = ws_services.Response{
			Success:      0,
			ErrorCode:    "json_syntax_error",
			ErrorDetails: fmt.Sprintf("Invalid JSON syntax at byte offset %d", syntaxErr.Offset),
		}
		// Check if the error is a JSON unmarshal type error
	} else if errors.As(err, &unmarshalTypeErr) {
		response = ws_services.Response{
			Success:      0,
			ErrorCode:    "json_type_error",
			ErrorDetails: fmt.Sprintf("Invalid JSON type for field '%s' at byte offset %d", unmarshalTypeErr.Field, unmarshalTypeErr.Offset),
		}
	} else {
		// For other error types, set a generic error response
		response = ws_services.Response{
			Success:      0,
			ErrorCode:    "internal_server_error",
			ErrorDetails: "An internal server error occurred.",
		}
	}

	// Write the error response with the provided status code
	writeErrResponse(w, statusCode, response)
}

// HandleSuccessResponse sends a JSON success response with optional headers.
func HandleSuccessResponse(w http.ResponseWriter, statusCode int, headers map[string]string, response interface{}, location string) {

	w.Header().Set("Content-Type", "application/json")

	// We don't want to cache API responses so the client receives most curent data
	w.Header().Set("Cache-Control", "max-age=0")

	// Conditionally set the Location header if provided (non-empty)
	if location != "" {
		w.Header().Set("Location", location)
	}

	// Add non-default headers
	for key, value := range headers {
		w.Header().Set(key, value)
	}

	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
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

// isDNSCompatible returns true if the provided name is DNS-compatible
func IsDNSCompatible(name string) bool {
	return dnsNameRegex.MatchString(name)
}
