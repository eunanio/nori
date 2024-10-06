package cmd

import (
	"os"

	"github.com/eunanio/nori/internal/console"
	"github.com/eunanio/nori/internal/deployment"
	"github.com/eunanio/nori/internal/futils"
	"github.com/google/uuid"
	"github.com/spf13/cobra"
)

var planCmd = &cobra.Command{
	Use:   "plan <tag>",
	Short: "Plan a deployment",
	Long:  `Plan a deployment of a module`,
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) != 1 {
			console.Error("Invalid number of arguments")
			return
		}

		tag, err := futils.ParseTagV2(args[0])
		if err != nil {
			console.Error("Error: Invalid tag")
		}
		if valuesFileFlag == "" {
			panic("values file required to plan deployments")
		}

		console.Debug("Values file: " + valuesFileFlag)

		if !futils.FileExists(valuesFileFlag) {
			console.Error("Error: Values file not found")
			return
		}

		if releaseFlag == "" {
			uuid, err := uuid.NewV7()
			if err != nil {
				console.Error("Error: Unable to generate release ID")
				return
			}
			releaseFlag = uuid.String()
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
			console.Error(err.Error())
			os.Exit(1)
		}

	},
}
