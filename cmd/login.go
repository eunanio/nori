package cmd

import (
	"github.com/eunanio/nori/internal/console"
	"github.com/eunanio/nori/internal/futils"
	"github.com/eunanio/nori/internal/oci"
	"github.com/spf13/cobra"
)

var LoginCmd = &cobra.Command{
	Use:   "login <registry>",
	Short: "Login to a registry",
	Long:  `Authencaites with the registry and stores the credentials`,
	Run: func(cmd *cobra.Command, args []string) {
		// Do Stuff Here
		if len(args) != 1 {
			console.Error("Invalid number of arguments")
			return
		}

		username := registryUsernameFlag
		password := registryPasswordFlag
		if registryPasswordStdinFlag {
			password = futils.GetStdin()
		}
		validateRegistryLogin(username, password, args[0])
		oci.Login(args[0], username, password)

	},
}

func validateRegistryLogin(username, password, addr string) bool {
	isValid := true
	if addr == "" {
		console.Error("Registry address is required")
	}

	if username == "" || password == "" || addr == "" {
		console.Error("Error: Username, password and registry address are required")
	}

	return isValid
}
