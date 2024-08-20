package cmd

import (
	"github.com/eunanio/nori/internal/deployment"
	"github.com/spf13/cobra"
)

var releaseCmd = &cobra.Command{
	Use:   "release",
	Short: "Release related commands",
	Long:  "Release related commands",
	Run:   func(cmd *cobra.Command, args []string) {},
}

var releaseListCmd = &cobra.Command{
	Use:   "list",
	Short: "Release related commands",
	Long:  "Release related commands",
	Run: func(cmd *cobra.Command, args []string) {
		deployment.ListReleases()
	},
}
