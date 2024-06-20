/*
Route: nori create -t --tag [tag] --path [path]
Package creates a new Oci compiant image from the module directory specified.
*/
package cmd

import (
	"github.com/eunanhardy/nori/internal/pkg"
	"github.com/spf13/cobra"
)

var packageCmd = &cobra.Command{
	Use:   "package [directory]",
	Short: "Package a module into an OCI image",
	Long:  `Package a module into an OCI image`,
	Run: func(cmd *cobra.Command, args []string) {
		pkg.PackageModule(packageTagFlag, args[0])
	},
}