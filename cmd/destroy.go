package cmd

import (
	"fmt"

	"github.com/eunanio/nori/internal/deployment"
	"github.com/spf13/cobra"
)

var destroyCmd = &cobra.Command{
	Use:   "destroy",
	Short: "Destory a release",
	Long:  "Destory a release and its associated resources",
	Run: func(cmd *cobra.Command, args []string) {
		// Do Stuff Here
		fmt.Println(args)
		if len(args) == 0 {
			fmt.Println("error: release ID required")
			return
		}

		err := deployment.Destory(args[0])
		if err != nil {
			fmt.Println(err.Error())
			panic(err)
		}
	},
}
