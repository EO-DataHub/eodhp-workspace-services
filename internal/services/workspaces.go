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

// Example request - will mature as we understand the requirements better
//
//	{
//	    "status": "created",
//	    "name": "jlangstone-new",
//	    "stores": {
//	        "object": [
//	            {
//	                "bucketName": "example-bucket"
//	            }
//	        ],
//	        "block": [
//	            {
//	                "name": "example-efs"
//	            }
//	        ]
//	    }
//	}
//
// Handles the creation of a workspace and its related components
func CreateWorkspaceService(workspaceDB *db.WorkspaceDB, w http.ResponseWriter, r *http.Request) {

	// Get the claims from the context
	claims, ok := r.Context().Value(middleware.ClaimsKey).(authn.Claims)

	if !ok {
		http.Error(w, "Unauthorized: invalid claims", http.StatusUnauthorized)
		return
	}

	// Parse and decode the request body into a MessagePayload object
	var messagePayload models.ReqMessagePayload
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
