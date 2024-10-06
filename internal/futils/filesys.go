package futils

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/eunanio/nori/internal/console"
	"github.com/eunanio/nori/internal/paths"
	"github.com/eunanio/nori/internal/spec"
	"gopkg.in/yaml.v2"
)

type ModuleMap struct {
	Modules map[string]string `json:"modules"`
}

func FileExists(filename string) bool {
	_, err := os.Stat(filename)
	return !os.IsNotExist(err)
}

func GetStdin() (msg string) {
	scanner := bufio.NewScanner(os.Stdin)
	if scanner.Scan() {
		msg = scanner.Text()
	}
	if err := scanner.Err(); err != nil {
		fmt.Fprintln(os.Stderr, "reading standard input:", err)
	}

	return msg
}

func ParseValuesFile(file string, config *spec.Config) (values map[string]interface{}, err error) {

	fileBytes, err := os.ReadFile(file)
	if err != nil {
		return nil, fmt.Errorf("Error reading values file: " + err.Error())
	}

	if filepath.Ext(file) == ".json" {
		err = json.Unmarshal(fileBytes, &values)
		if err != nil {
			return nil, err
		}
	} else if filepath.Ext(file) == ".yaml" || filepath.Ext(file) == ".yml" {
		err = yaml.Unmarshal(fileBytes, &values)
		if err != nil {
			return nil, err
		}
	} else {
		console.Error("Error: Unsupported values file type")
		return nil, fmt.Errorf("unsupported values file type")
	}
	for key, val := range config.Inputs {
		if _, ok := values[key]; !ok {
			if val.Default != nil {
				values[key] = val.Default
				continue
			}
			return nil, fmt.Errorf("missing required input: %s", key)
		}
	}

	if len(values) != 0 {
		for key, value := range values {
			switch v := value.(type) {
			case map[interface{}]interface{}:
				values[key] = convertMapKeysToStrings(v)
			case []interface{}:
				values[key] = convertSliceKeysToStrings(v)
			}
		}
	}

	return values, nil
}

func CreateOrUpdateIndex(tag *spec.Tag, sha string) error {
	sha = sha[7:]
	var index ModuleMap
	indexPath := paths.GetModuleMapPath()
	if FileExists(indexPath) {
		indexBytes, err := os.ReadFile(indexPath)
		if err != nil {
			return err
		}
		err = json.Unmarshal(indexBytes, &index)
		if err != nil {
			return err
		}
	}

	if index.Modules == nil {
		index.Modules = make(map[string]string)
	}

	// if _, ok := index.Modules[tag.String()]; ok {
	// 	return nil
	// }

	index.Modules[tag.String()] = sha
	indexBytes, err := json.Marshal(index)
	if err != nil {
		return err
	}
	os.WriteFile(indexPath, indexBytes, 0644)
	return nil
}

func RemoveIndexEntry(tag *spec.Tag) error {
	var index ModuleMap
	indexPath := paths.GetModuleMapPath()
	if FileExists(indexPath) {
		indexBytes, err := os.ReadFile(indexPath)
		if err != nil {
			return err
		}
		err = json.Unmarshal(indexBytes, &index)
		if err != nil {
			return err
		}
	}

	delete(index.Modules, tag.String())
	indexBytes, err := json.Marshal(index)
	if err != nil {
		return err
	}
	os.WriteFile(indexPath, indexBytes, 0644)
	return nil
}

func GetTaggedManifest(tag *spec.Tag) (*spec.Manifest, error) {
	var index ModuleMap
	indexPath := paths.GetModuleMapPath()
	if FileExists(indexPath) {
		indexBytes, err := os.ReadFile(indexPath)
		if err != nil {
			return nil, err
		}
		err = json.Unmarshal(indexBytes, &index)
		if err != nil {
			return nil, err
		}
	}

	if sha, ok := index.Modules[tag.String()]; ok {
		manifestPath := paths.GetBlobPathV2(sha)
		if FileExists(manifestPath) {
			manifestBytes, err := os.ReadFile(manifestPath)
			if err != nil {
				return nil, err
			}
			var manifest spec.Manifest
			err = json.Unmarshal(manifestBytes, &manifest)
			if err != nil {
				return nil, err
			}
			return &manifest, nil
		}

		return nil, nil
	}

	return nil, nil
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

func IsDebug() bool {
	_, ok := os.LookupEnv("NORI_DEBUG")
	return ok
}
