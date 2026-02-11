package cmd

import (
	"fmt"

	"aws-groups-manager/internal/version"
	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print build version",
	Run: func(_ *cobra.Command, _ []string) {
		fmt.Println(version.String())
	},
}
