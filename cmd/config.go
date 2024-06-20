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

	"github.com/eunanhardy/nori/internal/config"
	"github.com/eunanhardy/nori/internal/paths"
	"github.com/spf13/cobra"
)

var ConfigInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize nori configuration",
	Long:  `Initialize nori configuration`,
	Run: func(cmd *cobra.Command, args []string) {
		// Do stuff here
		config := config.Config{Runtime: "terraform"}
		configPath := paths.GetConfigPath()
		err := validateConfigFlags(&config)
		if err != nil {
			panic(err)
		}

		jsonBytes, err := json.MarshalIndent(config, "", "  ")
		if err != nil {
			panic("Error: Could not marshal config")
		}
		err = os.WriteFile(configPath, jsonBytes, 0644)
		if err != nil {
			panic("Error: Could not write config file")
		}
		fmt.Println("Nori configuration initialized successfully")
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