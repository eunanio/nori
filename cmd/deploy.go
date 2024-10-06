package cmd

import (
	"fmt"
	"os"

	"github.com/eunanio/nori/internal/console"
	"github.com/eunanio/nori/internal/deployment"
	"github.com/eunanio/nori/internal/futils"
	"github.com/spf13/cobra"
)

var deployCmd = &cobra.Command{
	Use:   "apply <release> <tag>",
	Short: "Deploy terraform package",
	Long:  `Deploy infrastructure from package`,
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) != 2 {
			console.Error("Invalid number of arguments")
			return
		}

		releaseFlag := args[0]
		vr := validateRelease(releaseFlag)
		if !vr {
			return
		}

		tag, err := futils.ParseTagV2(args[1])
		if err != nil {
			fmt.Println("Error parsing tag: ", err.Error())
		}

		if valuesFileFlag != "" {
			if futils.IsDebug() {
				fmt.Println("Using values file: ", valuesFileFlag)
			}

			if !futils.FileExists(valuesFileFlag) {
				fmt.Println("Error: Values file not found")
				return
			}
		}

		opts := deployment.DeploymentOpts{
			Tag:        tag,
			ValuesPath: valuesFileFlag,
			ApplyType:  deployment.TYPE_APPLY,
			ReleaseId:  releaseFlag,
		}

		err = deployment.Run(opts)
		if err != nil {
			fmt.Println("Error deploying: ", err.Error())
			os.Exit(1)
		}

	},
}
