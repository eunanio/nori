package futils

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/eunanio/nori/internal/paths"
)

func ListPackages() error {
	// list all the images
	var moduleMap ModuleMap
	data, err := os.ReadFile(paths.GetModuleMapPath())
	if err != nil {
		return err
	}

	err = json.Unmarshal(data, &moduleMap)
	if err != nil {
		return err
	}

	if len(moduleMap.Modules) > 0 {
		fmt.Println("--- Packages ---")
		for k, _ := range moduleMap.Modules {
			fmt.Println(k)
		}
	} else {
		fmt.Println("No packages found")
	}

	return nil
}
