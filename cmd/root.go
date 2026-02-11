package cmd

import (
	"os"

	"aws-groups-manager/internal/app"
	"github.com/spf13/cobra"
)

type rootOptions struct {
	profile string
	region  string
}

var opts rootOptions

var rootCmd = &cobra.Command{
	Use:   "aws-groups-manager",
	Short: "Manage IAM Identity Center groups from a TUI",
	RunE: func(cmd *cobra.Command, _ []string) error {
		cfg := app.StartConfig{
			Profile: opts.profile,
			Region:  opts.region,
		}
		return app.Run(cfg, os.Stdout)
	},
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.PersistentFlags().StringVar(&opts.profile, "profile", "", "AWS profile name")
	rootCmd.PersistentFlags().StringVar(&opts.region, "region", "", "AWS region")

	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(updateCmd)
	rootCmd.AddCommand(tuiCmd)
}
