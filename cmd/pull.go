package cmd

import (
	"github.com/eunanio/nori/internal/console"
	"github.com/eunanio/nori/internal/futils"
	"github.com/eunanio/nori/internal/pull"
	"github.com/spf13/cobra"
)

var pullCmd = &cobra.Command{
	Use:   "pull <tag>",
	Short: "Pull a tagged package from a registry",
	Long:  `Pull a tagged package from a oci compliant registry`,
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) == 0 {
			console.Error("No tag provided")
			return
		}

		tagInput := args[0]
		if tagInput == "" {
			panic("Error: No tag provided")
		}

		tag, err := futils.ParseTagV2(tagInput)
		if err != nil {
			panic(err)
		}
		pull.PullImage(tag, pullCreateFlag, ".")
	},
}
