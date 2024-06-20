package config

import (
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/eunanhardy/nori/internal/futils"
	"github.com/eunanhardy/nori/internal/paths"
)

func Load() *Config {
	configPath := paths.GetOrCreateHomePath()
	config := &Config{}
	filepath := fmt.Sprintf("%s/config.json", configPath)
	if !futils.FileExists(filepath) {
		fmt.Println("Config file does not exist")
		return nil
	}

	fileBytes, err := os.ReadFile(filepath); if err != nil {
		log.Fatal("Error reading config file: ", err)
		return nil
	}

	err = json.Unmarshal(fileBytes, config); if err != nil {
		log.Fatal("Error unmarshalling config file: ", err)
		return nil
	}

	return config
}