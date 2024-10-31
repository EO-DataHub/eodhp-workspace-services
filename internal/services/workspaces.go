package services

import (
	"encoding/json"

	"net/http"
	"time"

	"github.com/EO-DataHub/eodhp-workspace-services/api/middleware"
	"github.com/EO-DataHub/eodhp-workspace-services/db"
	"github.com/EO-DataHub/eodhp-workspace-services/internal/authn"
	"github.com/EO-DataHub/eodhp-workspace-services/models"
	"github.com/google/uuid"
)

func GetWorkspacesService(workspaceDB *db.WorkspaceDB, w http.ResponseWriter, r *http.Request) {

	// Get the claims from the context
	claims, ok := r.Context().Value(middleware.ClaimsKey).(authn.Claims)
	if !ok {
		http.Error(w, "Unauthorized: invalid claims", http.StatusUnauthorized)
		return
	}

	// Retrieve user workspaces based on the username
	// TODO: retrieve workspaces based on the member group
	workspaces, err := workspaceDB.GetUserWorkspaces(claims.Username)
	if err != nil {

		// errorResponse := models.ErrorResponse{
		// 	Status:  "error",
		// 	Message: err.Error(),
		// }

		// w.Header().Set("Content-Type", "application/json")
		// w.WriteHeader(http.StatusInternalServerError) // HTTP 500 Internal Server Error
		// return

		workspaceDB.Log.Error().Err(err).Msg("Failed to retrieve workspaces for user")
		w.WriteHeader(http.StatusInternalServerError)
		//http.Error(w, "Failed to retrieve workspaces", http.StatusInternalServerError)
		return
	}

	// Prepare response with full details
	var responses []models.Workspace
	for _, ws := range workspaces {
		response := models.Workspace{
			ID:           ws.ID,
			Name:         ws.Name,
			Account:      ws.Account,
			AccountOwner: ws.AccountOwner,
			MemberGroup:  ws.MemberGroup,
			Status:       ws.Status,
			Timestamp:    ws.Timestamp,
			Stores:       ws.Stores,
		}
		responses = append(responses, response)
	}

	// Encode and send the response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(struct {
		Workspaces []models.Workspace `json:"workspaces"`
	}{Workspaces: responses}); err != nil {
		workspaceDB.Log.Error().Err(err).Msg("Failed to encode response")
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

// Handles the creation of a workspace and its related components
func CreateWorkspaceService(workspaceDB *db.WorkspaceDB, w http.ResponseWriter, r *http.Request) {

	// Get the claims from the context
	claims, ok := r.Context().Value(middleware.ClaimsKey).(authn.Claims)
	if !ok {
		http.Error(w, "Unauthorized: invalid claims", http.StatusUnauthorized)
		return
	}

	// Parse and decode the request body into a MessagePayload object
	var messagePayload models.Workspace
	if err := json.NewDecoder(r.Body).Decode(&messagePayload); err != nil {
		workspaceDB.Log.Error().Err(err).Msg("Invalid request payload")
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	// Add the claims to the message payload
	messagePayload.AccountOwner = claims.Username
	messagePayload.Account = uuid.New()        // TODO: will be replaced with the actual account ID
	messagePayload.MemberGroup = "placeholder" // TODO: will be replaced with the actual member group from Keycloak

	// Add the timestamp to the message payload to track the state of the workspace request
	messagePayload.Timestamp = time.Now().Unix()

	// Create the workspace transaction
	tx, err := workspaceDB.InsertWorkspace(&messagePayload)
	if err != nil {
		http.Error(w, "Failed to insert workspace", http.StatusInternalServerError)
		return
	}

	// Publish the message to be picked up by the workspace manager
	err = workspaceDB.Events.Publish(messagePayload)
	if err != nil {
		workspaceDB.Log.Error().Err(err).Msg("Failed to publish event.")

		tx.Rollback() // Rollback the transaction if the event fails to publish
		http.Error(w, "Failed to create workspace event", http.StatusInternalServerError)
		return
	}

	// Commit the transaction after sucessfully publishing the event
	if err := workspaceDB.CommitTransaction(tx); err != nil {
		http.Error(w, "Failed to commit workspace transaction", http.StatusInternalServerError)
		return
	}

	// Respond with 201 success
	w.WriteHeader(http.StatusCreated)

	// TODO: Respond with an appropriate JSON message
	w.Write([]byte("Workspace is creating"))
}
