package hcl

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/eunanio/nori/internal/spec"
	"github.com/hashicorp/hcl/v2/hclsimple"
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/gocty"
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
	attrs["source"] = fmt.Sprintf("./.nori/%s", name)
	modules[name] = attrs
	root["module"] = modules
	jsonBytes, err := json.Marshal(root)
	if err != nil {
		panic("Parsing attr Error:" + err.Error())
	}
	err = os.WriteFile(fmt.Sprintf("%s/%s", path, "main.tf.json"), jsonBytes, 0644)
	if err != nil {
		panic(err)
	}
}

func GenerateOutputsBlock(moduleName string, path string, outputs map[string]spec.ModuleOutputs) {
	root := make(map[string]interface{})
	outputsMap := make(map[string]interface{})
	for k, v := range outputs {
		var sensative bool
		var description string
		if v.Sensitive != nil {
			sensative = *v.Sensitive
		}
		if v.Description != nil {
			description = *v.Description
		}

		outputsMap[k] = map[string]interface{}{
			"value": fmt.Sprintf("${module.%s.%s}", moduleName, k),
		}

		if v.Sensitive != nil {
			outputsMap[k].(map[string]interface{})["sensitive"] = sensative
		}
		if v.Description != nil {
			outputsMap[k].(map[string]interface{})["description"] = description
		}
	}

	root["output"] = outputsMap
	jsonBytes, err := json.Marshal(root)
	if err != nil {
		fmt.Println("Error marshalling outputs: ", err)
		os.Exit(1)
	}

	err = os.WriteFile(fmt.Sprintf("%s/outputs.tf.json", path), jsonBytes, 0644)
	if err != nil {
		fmt.Println("Error writing outputs: ", err)
		os.Exit(1)
	}
}

func ParseModuleConfig(root string) (*ModuleConfig, error) {
	var moduleConfig ModuleConfig
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && filepath.Ext(path) == ".tf" {
			bytes, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			err = ParseHCLBytes(bytes, &moduleConfig)
			if err != nil {
				return err
			}

		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	return &moduleConfig, nil
}

func ParseHCLBytes(bytes []byte, moduleConfig *ModuleConfig) error {
	var fileConfig ModuleConfig
	err := hclsimple.Decode("main.hcl", bytes, nil, &fileConfig)
	if err != nil {
		return err
	}

	for idx, input := range fileConfig.Inputs {
		if input.Default != nil {
			d := input.Default
			value, err := decodeHCLValue(*d)
			if err != nil {
				return err
			}
			fileConfig.Inputs[idx].DefaultValue = value
		}

	}

	if len(fileConfig.Inputs) > 0 {
		moduleConfig.Inputs = append(moduleConfig.Inputs, fileConfig.Inputs...)
	}

	if len(fileConfig.Outputs) > 0 {
		moduleConfig.Outputs = append(moduleConfig.Outputs, fileConfig.Outputs...)
	}

	if len(fileConfig.Resources) > 0 {
		moduleConfig.Resources = append(moduleConfig.Resources, fileConfig.Resources...)
	}

	return nil
}

func decodeHCLValue(value cty.Value) (interface{}, error) {
	if value.Type().IsPrimitiveType() {
		switch value.Type() {
		case cty.String:
			return value.AsString(), nil
		case cty.Number:
			var num float64
			if err := gocty.FromCtyValue(value, &num); err != nil {
				return nil, err
			}
			return num, nil
		case cty.Bool:
			var b bool
			if err := gocty.FromCtyValue(value, &b); err != nil {
				return nil, err
			}
			return b, nil

		default:
			return nil, fmt.Errorf("unsupported type: %v", value.Type())
		}
	}

	if value.Type().IsTupleType() {
		var values []interface{}
		for _, v := range value.AsValueSlice() {
			val, err := decodeHCLValue(v)
			if err != nil {
				return nil, err
			}
			values = append(values, val)
		}
		return values, nil
	}

	if value.Type().IsMapType() {
		fmt.Println("testtest")
		m := make(map[string]interface{})
		err := gocty.FromCtyValue(value, &m)
		if err != nil {
			return nil, err
		}
		return m, nil
	}

	if value.Type().IsObjectType() {
		m := make(map[string]interface{})
		for k, v := range value.AsValueMap() {
			val, err := decodeHCLValue(v)
			if err != nil {
				return nil, err
			}
			m[k] = val
		}
		return m, nil
	}

	return nil, fmt.Errorf("unsupported type: %v", value.Type())

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
