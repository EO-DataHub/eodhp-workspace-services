package services

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/EO-DataHub/eodhp-workspace-services/api/middleware"
	"github.com/EO-DataHub/eodhp-workspace-services/db"
	"github.com/EO-DataHub/eodhp-workspace-services/internal/authn"
	"github.com/EO-DataHub/eodhp-workspace-services/internal/events"
	"github.com/google/uuid"
)

// Example request
//
//	{
//	    "status": "creating",
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

	claims, ok := r.Context().Value(middleware.ClaimsKey).(authn.Claims)

	if !ok {
		http.Error(w, "Unauthorized: invalid claims", http.StatusUnauthorized)
		return
	}

	fmt.Println("Claim username inside API: ", claims.Username)

	// Parse and decode the request body into a WorkspaceRequest object
	var messagePayload events.MessagePayload
	if err := json.NewDecoder(r.Body).Decode(&messagePayload); err != nil {
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		log.Println("Error decoding request body:", err)
		return
	}

	messagePayload.AccountOwner = claims.Username
	messagePayload.Timestamp = time.Now().Unix()

	// placeholder assignments
	messagePayload.Account = uuid.New()        // will be replaced with the actual account ID from clai
	messagePayload.MemberGroup = "placeholder" // will be replaced with the actual member group from Keycloak

	// Now send the to the workspace manager and wait for an ACK
	// Send notification via the event system
	err := workspaceDB.Events.Publish(messagePayload)
	if err != nil {
		workspaceDB.Log.Error().Err(err).Msg("Failed to send event notification")
		http.Error(w, "Failed to create workspace event", http.StatusInternalServerError)
		return
	}

	ack, nil := workspaceDB.Events.ReceiveAck(messagePayload.Name)
	if err != nil {
		workspaceDB.Log.Error().Err(err).Msg("Failed to receive ACK for workspace creation")
		http.Error(w, "Failed to receive ACK", http.StatusInternalServerError)
		return
	}

	if ack.Status != "created" {
		// Respond with error
		http.Error(w, "Failed to create workspace", http.StatusInternalServerError)
		log.Println("Error inserting workspace. status: ", ack.Status)
		return
	}

	fmt.Println("ACK received: ", ack.Status)

	// Respond with success
	w.WriteHeader(http.StatusCreated)
	w.Write([]byte("Workspace created successfully"))
}
