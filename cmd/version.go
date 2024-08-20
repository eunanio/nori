package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

const VERSION = "0.4.0"

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: fmt.Sprintf("nori version: %s", VERSION),
	Long:  fmt.Sprintf("nori version: %s", VERSION),
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Nori version: ", VERSION)
	},
}
