package hclparse

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/hashicorp/hcl/v2/hclsimple"
)

func GenerateModuleBlock(name, path string, attrs map[string]interface{}) {
	modules := make(map[string]interface{})
	root := make(map[string]interface{})
	attrs["source"] = fmt.Sprintf("./.nori/%s",name)
	modules[name] = attrs
	root["module"] = modules
	jsonBytes, err := json.Marshal(root); if err != nil {
		panic(err)
	}
	err = os.WriteFile(fmt.Sprintf("%s/%s",path,"main.tf.json"), jsonBytes, 0644); if err != nil {
		panic(err)
	}
}

func ParseModuleConfig(root string) (*ModuleConfig,error) {
	var moduleConfig ModuleConfig
    filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
        if err != nil {
            return err
        }
        if !info.IsDir() && filepath.Ext(path) == ".tf" {
            bytes, err := os.ReadFile(path)
            if err != nil {
                return err
            }
			var fileConfig ModuleConfig
			err = hclsimple.Decode("main.hcl",bytes,nil,&fileConfig); if err != nil {
				slog.Warn("error resolving hcl input")
			}
			if len(fileConfig.Inputs ) > 0 {
				moduleConfig.Inputs = append(moduleConfig.Inputs, fileConfig.Inputs...)
			}

			if len(fileConfig.Outputs) > 0 {
				moduleConfig.Outputs = append(moduleConfig.Outputs, fileConfig.Outputs...)
			}
        }
        return nil
    })

	return &moduleConfig,nil
}