package cmd

import (
	"context"

	"aws-groups-manager/internal/updater"
	"github.com/spf13/cobra"
)

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Self-update from latest GitHub release",
	RunE: func(_ *cobra.Command, _ []string) error {
		return updater.Run(context.Background())
	},
}
