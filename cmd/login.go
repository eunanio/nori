package cmd

import (
	"github.com/eunanhardy/nori/internal/futils"
	"github.com/eunanhardy/nori/internal/oci"
	"github.com/spf13/cobra"
)

var LoginCmd = &cobra.Command{
  Use:   "login",
  Short: "Login to a registry",
  Long:  `Authencaites with the registry and stores the credentials`,
  Run: func(cmd *cobra.Command, args []string) {
    // Do Stuff Here
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
    panic("Error: Registry address is required")
  }

  if username == "" || password == "" || addr == ""{
    panic("Error: Username, password and registry address are required")
  }

  return isValid
}

