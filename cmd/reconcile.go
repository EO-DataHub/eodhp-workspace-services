package cmd

import (
	ws_manager "github.com/EO-DataHub/eodhp-workspace-manager/models"
	"github.com/EO-DataHub/eodhp-workspace-services/internal/events"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

var reconcileCmd = &cobra.Command{
	Use:   "reconcile",
	Short: "If the workspace exists in the database but not in Kubernetes, create it",
	Run: func(cmd *cobra.Command, args []string) {

		// Load the config, initialize the database and set up logging
		commonSetUp()

		// Initialize event publisher
		publisher, err := events.NewEventPublisher(appCfg.Pulsar.URL, appCfg.Pulsar.TopicProducer)
		if err != nil {
			log.Fatal().Err(err).Msg("Failed to initialize event publisher")
		}
		defer publisher.Close()

		// Get all the workspaces from the database
		workspaces, err := workspaceDB.GetAllWorkspaces()
		if err != nil {
			log.Fatal().Err(err).Msg("Error fetching workspace names")
		}

		log.Info().Msg("Starting reconciliation process...")

		// Iterate through each workspace and send its settings
		for _, workspaceName := range workspaces {
			log.Info().Msgf("Publishing workspace settings for: %s", workspaceName)

			// Construct minimal workspace settings
			wsSettings := ws_manager.WorkspaceSettings{
				Name:   workspaceName,
				Status: "creating",
				Stores: &[]ws_manager.Stores{
					{
						Object: []ws_manager.ObjectStore{
							{Name: workspaceName + "-object-store"},
						},
						Block: []ws_manager.BlockStore{
							{Name: "block-store"},
						},
					},
				},
			}

			err = publisher.Publish(wsSettings)
			if err != nil {
				log.Error().Err(err).Msgf("Failed to publish workspace settings for: %s", workspaceName)
			} else {
				log.Info().Msgf("Successfully published workspace settings for: %s", workspaceName)
			}
		}

		log.Info().Msg("Workspace publishing process completed.")
	},
}

func init() {
	rootCmd.AddCommand(reconcileCmd)
}
