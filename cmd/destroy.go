package cmd

import (
	"fmt"

	"github.com/eunanio/nori/internal/console"
	"github.com/eunanio/nori/internal/deployment"
	"github.com/spf13/cobra"
)

var destroyCmd = &cobra.Command{
	Use:   "destroy <release>",
	Short: "Destory a release",
	Long:  "Destory a release and its associated resources",
	Run: func(cmd *cobra.Command, args []string) {

		if len(args) != 1 {
			console.Error("release ID required")
			return
		}

		releaseId := args[0]
		ok := validateRelease(releaseId)
		if !ok {
			fmt.Println("error: invalid release ID")
			return
		}

		err := deployment.Destory(releaseId)
		if err != nil {
			console.Error("error: " + err.Error())
			return
		}
	},
}
