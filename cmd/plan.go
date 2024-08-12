package cmd

import (
	"fmt"
	"os"

	"github.com/eunanio/nori/internal/deployment"
	"github.com/eunanio/nori/internal/futils"
	"github.com/spf13/cobra"
)

var planCmd = &cobra.Command{
	Use:   "plan",
	Short: "Plan a deployment",
	Long:  `Plan a deployment of a module`,
	Run: func(cmd *cobra.Command, args []string) {
		tag, err := futils.ParseTagV2(args[0])
		if err != nil {
			fmt.Println("Error parsing tag: ", err.Error())
		}
		if valuesFileFlag == "" {
			panic("values file required to plan deployments")
		}
		fmt.Println("Using values file: ", valuesFileFlag)
		if !futils.FileExists(valuesFileFlag) {
			fmt.Println("Error: Values file not found")
			return
		}

		opts := deployment.DeploymentOpts{
			Tag:          tag,
			ValuesPath:   valuesFileFlag,
			ApplyType:    deployment.TYPE_PLAN,
			ReleaseId:    releaseFlag,
			ProviderFile: providerFileFlag,
		}

		err = deployment.Run(opts)
		if err != nil {
			fmt.Println(err.Error())
			os.Exit(1)
		}

	},
}
