package hcl

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/eunanhardy/nori/internal/spec"
	"github.com/hashicorp/hcl/v2/hclsimple"
)

func GenerateModuleBlock(name, path string, attrs map[string]interface{}) {
	if len(attrs) != 0 {
		for key, value := range attrs {
			switch v := value.(type) {
			case map[interface{}]interface{}:
				attrs[key] = convertMapKeysToStrings(v)
			case []interface{}:
				attrs[key] = convertSliceKeysToStrings(v)
			}
		}
	}

	modules := make(map[string]interface{})
	root := make(map[string]interface{})
	attrs["source"] = fmt.Sprintf("./.nori/%s",name)
	modules[name] = attrs
	root["module"] = modules
	jsonBytes, err := json.Marshal(root); if err != nil {
		panic("Parsing attr Error:"+ err.Error())
	}
	err = os.WriteFile(fmt.Sprintf("%s/%s",path,"main.tf.json"), jsonBytes, 0644); if err != nil {
		panic(err)
	}
}

func GenerateOutputsBlock(moduleName string, path string, outputs map[string]spec.ModuleOutputs){
	root := make(map[string]interface{})
	outputsMap := make(map[string]interface{})
	for k := range outputs {
		outputsMap[k] = map[string]interface{}{
			"value": fmt.Sprintf("${module.%s.%s}",moduleName,k),
		}
	}

	root["output"] = outputsMap
	jsonBytes, err := json.Marshal(root); if err != nil {
		fmt.Println("Error marshalling outputs: ", err)
		os.Exit(1)
	}

	err = os.WriteFile(fmt.Sprintf("%s/outputs.tf.json",path), jsonBytes, 0644); if err != nil {
		fmt.Println("Error writing outputs: ", err)
		os.Exit(1)
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

func convertMapKeysToStrings(input map[interface{}]interface{}) map[string]interface{} {
	output := make(map[string]interface{})
	for key, value := range input {
		strKey := fmt.Sprintf("%v", key)
		switch v := value.(type) {
		case map[interface{}]interface{}:
			output[strKey] = convertMapKeysToStrings(v)
		case []interface{}:
			output[strKey] = convertSliceKeysToStrings(v)
		default:
			output[strKey] = value
		}
	}
	return output
}

func convertSliceKeysToStrings(input []interface{}) []interface{} {
	output := make([]interface{}, len(input))
	for i, value := range input {
		switch v := value.(type) {
		case map[interface{}]interface{}:
			output[i] = convertMapKeysToStrings(v)
		case []interface{}:
			output[i] = convertSliceKeysToStrings(v)
		default:
			output[i] = value
		}
	}
	return output
}