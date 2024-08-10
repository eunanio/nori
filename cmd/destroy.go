package cmd

import (
	"github.com/eunanhardy/nori/internal/deployment"
	"github.com/spf13/cobra"
)

var destroyCmd = &cobra.Command{
	Use:   "destroy",
	Short: "Destory a release",
	Long:  "Destory a release and its associated resources",
	Run: func(cmd *cobra.Command, args []string) {
		// Do Stuff Here
		err := deployment.Destory(args[0])
		if err != nil {
			panic(err)
		}
	},
}
