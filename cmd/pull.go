package cmd

import (
	"github.com/eunanhardy/nori/internal/e"
	"github.com/eunanhardy/nori/internal/futils"
	"github.com/eunanhardy/nori/internal/pull"
	"github.com/spf13/cobra"
)

var pullCmd = &cobra.Command{
	Use:   "pull",
	Short: "Pull a tagged package from a registry",
	Long:  `Pull a tagged package from a oci compliant registry`,
	Run: func(cmd *cobra.Command, args []string) {
		tagInput := args[0]
		if tagInput == "" {
			panic("Error: No tag provided")
		}

		tag, err := futils.ParseImageTag(tagInput)
		e.Fatal(err, "Error: Invalid tag")
		pull.PullImage(tag,pullCreateFlag,".")
	},
}