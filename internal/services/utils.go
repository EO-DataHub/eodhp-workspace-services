package services

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/EO-DataHub/eodhp-workspace-services/models"
	"github.com/lib/pq"
)

// writeJSONResponse writes a JSON response with a specific status code
func WriteErrResponse(w http.ResponseWriter, statusCode int, response interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

// WriteDatabaseErrorResponse handles pq.Error and writes JSON error responses
func HandleErrResponse(w http.ResponseWriter, statusCode int, err error) {
	var pqErr *pq.Error
	var response models.Response

	if errors.As(err, &pqErr) {
		response = models.Response{
			Success:      0,
			ErrorCode:    pqErr.Code.Name(),
			ErrorDetails: pqErr.Message,
		}

	} else {
		response = models.Response{
			Success:      0,
			ErrorDetails: err.Error(),
		}
		//http.Error(w, "Internal server error", http.StatusInternalServerError)
	}

	WriteErrResponse(w, statusCode, response)
}

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
