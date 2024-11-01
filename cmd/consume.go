package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var consumeCmd = &cobra.Command{
	Use:   "consume",
	Short: "Run the Pulsar consumer to process events in the workspace-status topic",
	Run: func(cmd *cobra.Command, args []string) {

		// TODO: Implement the consume command
		fmt.Println("Starting Pulsar consumer")
	},
}

func init() {
	rootCmd.AddCommand(consumeCmd)
}
