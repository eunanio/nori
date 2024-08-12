package cmd

import (
	"github.com/eunanio/nori/internal/futils"
	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "ls",
	Short: "List all the packaged modules",
	Long:  "List all the packaged modules",
	Run: func(cmd *cobra.Command, args []string) {
		err := futils.ListPackages()
		if err != nil {
			panic(err)
		}
	},
}
