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
	"github.com/rs/zerolog/log"
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
	messagePayload.Account = uuid.New()        // TODO: will be replaced with the actual account ID from claim
	messagePayload.MemberGroup = "placeholder" // TODO: will be replaced with the actual member group from Keycloak

	// Add the timestamp to the message payload to track the state of the workspace request at other end
	messagePayload.Timestamp = time.Now().Unix()

	// Publish the message to be picked up by the workspace manager
	err := workspaceDB.Events.Publish(messagePayload)
	if err != nil {
		workspaceDB.Log.Error().Err(err).Msg("Failed to publish event.")
		http.Error(w, "Failed to create workspace event", http.StatusInternalServerError)
		return
	}

	// Receive the ACK for the workspace creation
	ack, nil := workspaceDB.Events.ReceiveAck(messagePayload)
	if err != nil {
		workspaceDB.Log.Error().Err(err).Msg("Failed to receive ACK for workspace creation")
		http.Error(w, "Failed to receive ACK for workspace creation", http.StatusInternalServerError)
		return
	}

	// Check if the workspace was created successfully. Any non compliant status will be returned as an error
	if ack.MessagePayload.Status != "created" {
		workspaceDB.Log.Error().Err(err).Msg("Error creating workspace.")
		log.Error().Str("status", ack.MessagePayload.Status).Msg("Error creating workspace")
		http.Error(w, "Failed to create workspace", http.StatusInternalServerError)
		return
	}

	// Add the Workspace metadata response to the database
	err = workspaceDB.InsertWorkspace(ack)
	if err != nil {
		http.Error(w, "Failed to insert workspace", http.StatusInternalServerError)
		return
	}

	// Respond with success
	w.WriteHeader(http.StatusCreated)
	w.Write([]byte("Workspace created successfully"))
}
