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
	if !FileExists(paths.GetModuleMapPath()) {
		fmt.Println("No packages found")
		return nil
	}

	data, err := os.ReadFile(paths.GetModuleMapPath())
	if err != nil {
		return err
	}

	err = json.Unmarshal(data, &moduleMap)
	if err != nil {
		return err
	}

	if len(moduleMap.Modules) > 0 {
		fmt.Println("\tPACKAGES")
		for k, _ := range moduleMap.Modules {
			fmt.Println(k)
		}
	} else {
		fmt.Println("No packages found")
	}

	return nil
}
