/*
Push a tagged package to a oci compliant registry
*/
package cmd

import (
	"github.com/eunanhardy/nori/internal/futils"
	"github.com/eunanhardy/nori/internal/push"
	"github.com/spf13/cobra"
)


var pushCmd = &cobra.Command{
	Use:   "push",
	Short: "Push a tagged package to a registry",
	Long:  `Push a tagged package to a oci compliant registry`,
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) == 0 {
			panic("Error: No tag provided")
		}

		tagStr := args[0]
		tag, err := futils.ParseImageTag(tagStr); if err != nil {
			panic("Error: Invalid tag")
		}

		push.PushImage(tag,insecureFlag)
	},
}