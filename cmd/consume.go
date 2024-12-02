package cmd

import (
	"context"
	"encoding/json"
	"fmt"

	ws_manager "github.com/EO-DataHub/eodhp-workspace-manager/models"
	"github.com/EO-DataHub/eodhp-workspace-services/internal/events"
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
		consumer, err := events.NewEventConsumer(config.Pulsar.URL, config.Pulsar.TopicConsumer, config.Pulsar.Subscription)
		if err != nil {
			log.Fatal().Err(err).Msg("Failed to initialize event consumer")
		}
		defer consumer.Close()

		// Consume messages
		for {
			fmt.Println("Waiting for messages...")
			msg, err := consumer.ReceiveMessage(context.Background())
			if err != nil {
				log.Error().Err(err).Msg("Error receiving message")
				continue
			}

			// convert into
			fmt.Println("Received message: ", string(msg.Payload()))

			// Unmarshal the JSON message into WorkspaceStatus struct
			var workspaceStatus ws_manager.WorkspaceStatus
			err = json.Unmarshal([]byte(msg.Payload()), &workspaceStatus)
			if err != nil {
				fmt.Printf("Error unmarshaling JSON: %v\n", err)
				return
			}

			err = workspaceDB.UpdateWorkspaceStatus(workspaceStatus)
			if err != nil {
				log.Fatal().Err(err).Msg("Failed to update workspace status")
			}
			// Process message and update database
			consumer.Ack(msg)
		}

	},
}

func init() {
	rootCmd.AddCommand(consumeCmd)
}
