/*
Route: nori create -t --tag [tag] --path [path]
Package creates a new Oci compiant image from the module directory specified.
*/
package cmd

import (
	"fmt"

	"github.com/eunanio/nori/internal/console"
	"github.com/eunanio/nori/internal/futils"
	"github.com/eunanio/nori/internal/pkg"
	"github.com/spf13/cobra"
)

var packageCmd = &cobra.Command{
	Use:   "package <tag> <directory>",
	Short: "Package a terraform module",
	Long:  `Package a terraform module for distribution`,
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) != 2 {
			console.Error("Invalid number of arguments")
			return
		}

		if args[0] == "" {
			fmt.Println("Error: Invalid tag")
			return
		}

		if args[1] == "" {
			fmt.Println("Error: Invalid path")
			return
		}

		tag, err := futils.ParseTagV2(args[0])
		if err != nil {
			fmt.Println("Error parsing tag: ", err)
			return
		}
		err = pkg.PackageModuleV2(tag, args[1])
		if err != nil {
			fmt.Println("Error packaging module: ", err.Error())
			return
		}
	},
}
