package services

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/EO-DataHub/eodhp-workspace-services/models"
	"github.com/lib/pq"
)

// HandleErrResponse formats and sends error details, including pq.Error specifics.
func HandleErrResponse(w http.ResponseWriter, statusCode int, err error) {
	var pqErr *pq.Error
	var response models.Response

	// Check if the error is a PostgreSQL-specific error (pq.Error)
	if errors.As(err, &pqErr) {
		response = models.Response{
			Success:      0,
			ErrorCode:    pqErr.Code.Name(), // PostgreSQL error code name
			ErrorDetails: pqErr.Message,     // Detailed error message
		}

	} else {
		// For other error types, set a generic error response
		response = models.Response{
			Success:      0,
			ErrorDetails: err.Error(),
		}
	}

	// Write the error response with the provided status code
	writeErrResponse(w, statusCode, response)
}

// HandleSuccessResponse sends a JSON success response with optional headers.
func HandleSuccessResponse(w http.ResponseWriter, statusCode int, headers map[string]string, response interface{}) {
	w.Header().Set("Content-Type", "application/json")
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
	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}
