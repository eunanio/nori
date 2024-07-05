/*
Route: nori create -t --tag [tag] --path [path]
Package creates a new Oci compiant image from the module directory specified.
*/
package cmd

import (
	"fmt"

	"github.com/eunanhardy/nori/internal/futils"
	"github.com/eunanhardy/nori/internal/pkg"
	"github.com/spf13/cobra"
)

var packageCmd = &cobra.Command{
	Use:   "package [directory]",
	Short: "Package a module into an OCI image",
	Long:  `Package a module into an OCI image`,
	Run: func(cmd *cobra.Command, args []string) {
		tag, err := futils.ParseTagV2(packageTagFlag)
		if err != nil {
			fmt.Println("Error parsing tag: ", err)
			return
		}
		err  = pkg.PackageModuleV2(tag, args[0])
		if err != nil {
			fmt.Println("Error packaging module: ", err.Error())
			return
		}
	},
}