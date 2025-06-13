package cmd

import (
	"context"
	"encoding/json"
	"fmt"

	ws_manager "github.com/EO-DataHub/eodhp-workspace-manager/models"
	"github.com/EO-DataHub/eodhp-workspace-services/internal/events"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

var consumeCmd = &cobra.Command{
	Use:   "consume",
	Short: "Run the Pulsar consumer to process events in the workspace-status topic",
	Run: func(cmd *cobra.Command, args []string) {

		// Load the config, initialize the database and set up logging
		commonSetUp()

		// Initialize event consumer
		consumer, err := events.NewEventConsumer(appCfg.Pulsar.URL, appCfg.Pulsar.TopicConsumer, appCfg.Pulsar.Subscription)
		if err != nil {
			log.Fatal().Err(err).Msg("Failed to initialize event consumer")
		}
		defer consumer.Close()

		// Consume messages
		for {
			log.Info().Msg("Waiting for messages...")

			msg, err := consumer.ReceiveMessage(context.Background())
			if err != nil {
				log.Error().Err(err).Msg("Error receiving message")
				continue
			}

			log.Info().Str("status", string(msg.Payload())).Msg("Received message")

			// Unmarshal the JSON message into WorkspaceStatus struct
			var workspaceStatus ws_manager.WorkspaceStatus
			err = json.Unmarshal([]byte(msg.Payload()), &workspaceStatus)
			if err != nil {
				log.Error().Err(err).Msg("Error unmarshaling JSON")
				continue
			}

			if workspaceStatus.State == "Deleting" {
				err := deleteWorkspace(workspaceStatus)
				if err != nil {
					log.Error().Err(err).Msg("Failed to delete workspace")

					// Nack the message if there is an error and attempt redelivery
					consumer.Nack(msg)
					continue
				}
				consumer.Ack(msg)
			}

			// Get the workspace from the database and check if the incoming status is newer
			workspaceInDB, err := workspaceDB.GetWorkspace(workspaceStatus.Name)

			if err != nil {
				log.Error().Err(err).Str("workspace_name", workspaceStatus.Name).Msg("Workspace not found")

				// Acknowledge the message if the workspace is not found to discard it to prevent redelivery
				consumer.Ack(msg)
				continue
			}

			// Update the workspace in the database if the incoming status is newer
			if workspaceStatus.LastUpdated.After(workspaceInDB.LastUpdated) {

				// If the namespace is empty, delete the workspace
				if workspaceStatus.Namespace == "" {

					err := deleteWorkspace(workspaceStatus)
					if err != nil {
						log.Error().Err(err).Msg("Failed to delete workspace")

						// Nack the message if there is an error and attempt redelivery
						consumer.Nack(msg)
						continue
					}

				} else {
					// Update the workspace status
					err = workspaceDB.UpdateWorkspaceStatus(workspaceStatus)
					if err != nil {
						log.Error().Err(err).Msg("Failed to update workspace status")

						// Nack the message if there is an error and attempt redelivery
						consumer.Nack(msg)
						continue
					}
				}

				// Acknowledge the message if status is updated successfully
				consumer.Ack(msg)

			} else {
				log.Warn().Msg("Incoming status is older")

				// Discard the message if incoming status is older
				consumer.Ack(msg)
			}
		}
	},
}

// deleteWorkspace deletes a workspace by setting its status to 'Unavailable' in the database,
// removing its Keycloak group, and deleting any lingering secrets in AWS Secrets Manager.
func deleteWorkspace(wsStatus ws_manager.WorkspaceStatus) error {

	// Set the workspace as 'Unavailable' in the database
	err := workspaceDB.DisableWorkspace(wsStatus.Name)
	if err != nil {
		log.Error().Err(err).Msg("Failed to delete workspace")
		return err
	}

	// Get a token from keycloak so we can interact with it's API
	err = keycloakClient.GetToken()
	if err != nil {
		log.Error().Err(err).Msg("Failed to Authenticate with Keycloak")
		return err
	}

	_, err = keycloakClient.DeleteGroup(wsStatus.Name)
	if err != nil {
		log.Error().Err(err).Msg("Failed to delete Keycloak group")
		return err
	}

	// Remove any lingering secret from AWS Secrets Manager
	_, err = secretsManagerClient.DeleteSecret(context.Background(), &secretsmanager.DeleteSecretInput{
		SecretId:                   aws.String(fmt.Sprintf("ws-%s", wsStatus.Name)),
		ForceDeleteWithoutRecovery: aws.Bool(true),
	})
	if err != nil {
		log.Error().Err(err).Msg("Failed to delete AWS Secret manager secret")
		return err
	}

	return nil
}

func init() {
	rootCmd.AddCommand(consumeCmd)
}
