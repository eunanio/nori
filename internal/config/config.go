package config

import (
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/eunanio/nori/internal/futils"
	"github.com/eunanio/nori/internal/paths"
)

func Load() *Config {
	configPath := paths.GetOrCreateHomePath()
	config := &Config{}
	filepath := fmt.Sprintf("%s/config.json", configPath)
	if !futils.FileExists(filepath) {
		fmt.Println("Config file does not exist")
		return nil
	}

	fileBytes, err := os.ReadFile(filepath)
	if err != nil {
		log.Fatal("Error reading config file: ", err)
		return nil
	}

	err = json.Unmarshal(fileBytes, config)
	if err != nil {
		log.Fatal("Error unmarshalling config file: ", err)
		return nil
	}

	return config
}

func SetProject(project string) error {
	configPath := paths.GetConfigPath()
	if !futils.FileExists(configPath) {
		return fmt.Errorf("config file does not exist, ensure you have run nori init?")
	}

	configBytes, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("error reading config file: %s", err)
	}
	var config Config
	err = json.Unmarshal(configBytes, &config)
	if err != nil {
		return fmt.Errorf("error unmarshalling config file: %s", err)
	}

	config.Project = project
	configBytes, err = json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("error marshalling config: %s", err)
	}

	err = os.WriteFile(configPath, configBytes, 0644)
	if err != nil {
		return fmt.Errorf("error writing config file: %s", err)
	}

	return nil
}
