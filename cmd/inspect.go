package cmd

import (
	"github.com/eunanio/nori/internal/futils"
	"github.com/eunanio/nori/internal/inspect"
	"github.com/spf13/cobra"
)

var inspectCmd = &cobra.Command{
	Use:   "inspect <tag>",
	Short: "Inspect package information",
	Long:  `Inspect package information`,
	Run: func(cmd *cobra.Command, args []string) {
		tag, err := futils.ParseTagV2(args[0])
		if err != nil {
			panic(err)
		}

		inspect.GetImageInfo(tag)
	},
}
