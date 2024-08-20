/*
Route: nori config init --from-remote [url|s3]
Use to setup default configuration for nori to use, This will include the creation of
the .nori directory in the user's home directory and the creation of the config file
## Context
  - Must have a easy to use wizard for setting up the configuration
  - Must be able to setup configuration from remote sources, e.g. s3 or http
*/
package cmd

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"

	"github.com/eunanio/nori/internal/config"
	"github.com/eunanio/nori/internal/paths"
	"github.com/spf13/cobra"
)

var ConfigInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize nori configuration",
	Long:  `Initialize nori configuration`,
	Run: func(cmd *cobra.Command, args []string) {
		// Do stuff here
		configObj := config.Config{Runtime: "terraform", Project: "default"}
		configPath := paths.GetConfigPath()
		err := validateConfigFlags(&configObj)
		if err != nil {
			panic(err)
		}

		jsonBytes, err := json.MarshalIndent(configObj, "", "  ")
		if err != nil {
			panic("Error: Could not marshal config")
		}
		err = os.WriteFile(configPath, jsonBytes, 0644)
		if err != nil {
			panic("Error: Could not write config file")
		}
		if projectFlag != "" {
			err = config.SetProject(projectFlag)
			if err != nil {
				panic(err)
			}
		}
		fmt.Println("Nori configuration initialized successfully")
	},
}

var ConfigCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage nori configuration",
	Long:  `Manage nori configuration`,
	Run: func(cmd *cobra.Command, args []string) {
		if projectFlag != "" {
			fmt.Println("Setting project to: ", projectFlag)
			err := config.SetProject(projectFlag)
			if err != nil {
				panic(err)
			}
		}

		if configRemoteFlag != "" && backendRegionFlag != "" {
			err := config.SetBackendConfig(&configRemoteFlag, &backendRegionFlag)
			if err != nil {
				panic(err)
			}
		}
	},
}

var DisplayPorjectCmd = &cobra.Command{
	Use:   "project",
	Short: "Display the current project",
	Long:  `Display the current project`,
	Run: func(cmd *cobra.Command, args []string) {
		config := config.Load()
		if config == nil {
			fmt.Println("No project set")
			return
		}
		fmt.Println("Current project: ", config.Project)
	},
}

func validateConfigFlags(config *config.Config) error {
	if configRuntimeFlag != "" {
		if configRuntimeFlag != "terraform" && configRuntimeFlag != "tofu" {
			return fmt.Errorf("error: Invalid runtime specified")
		}

		config.Runtime = configRuntimeFlag
	}

	if configRemoteFlag != "" {
		valid_url, err := url.Parse(configRemoteFlag)
		if err != nil {
			return fmt.Errorf("error: Invalid remote source specified")
		}

		if valid_url.Scheme != "s3" {
			return fmt.Errorf("error: Invalid remote source specified")
		}

		if valid_url.Host == "" {
			return fmt.Errorf("error: Invalid remote source specified")
		}

		config.Remote = &configRemoteFlag
	}

	if backendRegionFlag != "" {
		config.Region = &backendRegionFlag
	}

	return nil

}
