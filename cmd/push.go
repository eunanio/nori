/*
Push a tagged package to a oci compliant registry
*/
package cmd

import (
	"github.com/eunanio/nori/internal/console"
	"github.com/eunanio/nori/internal/futils"
	"github.com/eunanio/nori/internal/push"
	"github.com/spf13/cobra"
)

var pushCmd = &cobra.Command{
	Use:   "push <tag>",
	Short: "Push a tagged package to a registry",
	Long:  `Push a tagged package to a oci compliant registry`,
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) != 1 {
			console.Error("No tag provided")
			return
		}

		tagStr := args[0]
		tag, err := futils.ParseTagV2(tagStr)
		if err != nil {
			panic("Error: Invalid tag")
		}

		push.PushImage(tag, insecureFlag)
	},
}
