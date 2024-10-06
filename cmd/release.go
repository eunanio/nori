package cmd

import (
	"regexp"

	"github.com/eunanio/nori/internal/console"
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

func validateRelease(releaseId string) bool {
	if releaseId == "" {
		console.Error("error: release ID required")
		return false
	}
	re := regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)
	if !re.MatchString(releaseId) {
		console.Error("error: invalid release ID, must be alphanumeric, _, or -")
		return false
	}

	return true

}
