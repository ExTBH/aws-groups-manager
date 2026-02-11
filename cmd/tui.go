package cmd

import (
	"os"

	"aws-groups-manager/internal/app"
	"github.com/spf13/cobra"
)

var tuiCmd = &cobra.Command{
	Use:   "tui",
	Short: "Run interactive TUI",
	RunE: func(_ *cobra.Command, _ []string) error {
		cfg := app.StartConfig{Profile: opts.profile, Region: opts.region}
		return app.Run(cfg, os.Stdout)
	},
}
