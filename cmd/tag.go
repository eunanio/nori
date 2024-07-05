package cmd

import (
	"github.com/eunanhardy/nori/internal/futils"
	"github.com/spf13/cobra"
)

var tagCmd = &cobra.Command{
	Use:   "tag [old] [new]",
	Short: "Rename a tag",
	Long:  `Rename a tag in the local registry`,
	Run:   func(cmd *cobra.Command, args []string) {
		oldTag, err := futils.ParseTagV2(args[0])
		if err != nil {
			panic(err)
		}

		newTag, err := futils.ParseTagV2(args[1])
		if err != nil {
			panic(err)
		}
		
		if oldTag.String() == newTag.String() {
			panic("old and new tags are the same")
		}

		err = futils.UpdateTag(oldTag.String(), newTag.String())
		if err != nil {
			panic(err)
		}
	},
}