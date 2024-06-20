package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	packageTagFlag string
	//packagePathFlag string
	registryUsernameFlag string
	registryPasswordFlag string
	registryPasswordStdinFlag bool
	pullCreateFlag bool
	valuesFileFlag string
	configRuntimeFlag string
	configRemoteFlag string
	insecureFlag bool
	backendRegionFlag string
	releaseFlag string
)

var rootCmd = &cobra.Command{
	Use:   "nori",
	Short: "Nori Cli",
	Long: `Nori helps you package, distribute and deploy Terraform modules`,
	Run: func(cmd *cobra.Command, args []string) {
		// Do Stuff Here
		fmt.Println("Nori allows you to package, distribute and deploy Terraform modules")
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	// Package
	rootCmd.AddCommand(packageCmd)
	packageCmd.Flags().StringVarP(&packageTagFlag,"tag", "t","","Tag for the image")
	//packageCmd.Flags().StringVarP(&packagePathFlag,"path", "p","","Path to the module directory")
	// Config init
	rootCmd.AddCommand(ConfigInitCmd)
	ConfigInitCmd.Flags().StringVarP(&configRuntimeFlag,"runtime", "r","","Runtime for the configuration, e.g. terraform or tofu")
	ConfigInitCmd.Flags().StringVarP(&configRemoteFlag,"backend", "b","","backend source for state configuration")
	ConfigInitCmd.Flags().StringVarP(&backendRegionFlag,"backend-region", "","","region for the backend. Only Support with S3 backend")
	// Login
	rootCmd.AddCommand(LoginCmd)
	LoginCmd.Flags().StringVarP(&registryUsernameFlag,"username", "u","","Username for the registry")
	LoginCmd.Flags().StringVarP(&registryPasswordFlag,"password", "p","","Password for the registry")
	LoginCmd.Flags().BoolVarP(&registryPasswordStdinFlag,"password-stdin", "",false,"Use the password from stdin")
	// Push
	rootCmd.AddCommand(pushCmd)
	pushCmd.Flags().BoolVarP(&insecureFlag,"insecure", "i",false,"Allow insecure registry communication")
	// Pull
	rootCmd.AddCommand(pullCmd)
	pullCmd.Flags().BoolVarP(&pullCreateFlag,"create", "c",false,"Exports the pulled image to the local working directory")
	// Plan
	rootCmd.AddCommand(planCmd)
	planCmd.Flags().StringVarP(&valuesFileFlag,"values", "v","","Values file for the deployment")
	planCmd.Flags().StringVarP(&releaseFlag,"release", "r","","Release ID for the deployment")
	// Deploy
	rootCmd.AddCommand(deployCmd)
	deployCmd.Flags().StringVarP(&valuesFileFlag,"values", "v","","Values file for the deployment")
	deployCmd.Flags().StringVarP(&releaseFlag,"release", "r","","Release ID for the deployment")
}