package cmd

import (
	"fmt"
	"os"

	"github.com/eunanhardy/nori/internal/deployment"
	"github.com/eunanhardy/nori/internal/futils"
	"github.com/spf13/cobra"
)

var deployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "Deploy infrastructure",
	Long:  `Deploy infrastructure from image`,
	Run: func(cmd *cobra.Command, args []string) {
		tag, err := futils.ParseImageTag(args[0]); if err != nil {
			fmt.Println("Error parsing tag: ", err.Error())
		}

		if valuesFileFlag != "" {
			fmt.Println("Using values file: ", valuesFileFlag)
			if !futils.FileExists(valuesFileFlag) {
				fmt.Println("Error: Values file not found")
				return
			}
		}

		opts := deployment.DeploymentOpts{
			Tag: tag,
			ValuesPath: valuesFileFlag,
			ApplyType: deployment.TYPE_APPLY,
			ReleaseId: releaseFlag,
		}

		err = deployment.Run(opts)
		if err != nil {
			fmt.Println("Error deploying: ", err.Error())
			os.Exit(1)
		}

	},
}