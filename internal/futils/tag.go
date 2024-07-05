package futils

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"

	"github.com/eunanhardy/nori/internal/paths"
	"github.com/eunanhardy/nori/internal/spec"
)

func ParseTagV2(tag string) (*spec.Tag, error) {
	pattern := `^(?:(?P<host>[a-zA-Z0-9.-]+(?::[0-9]+)?)\/)?(?:(?P<namespace>[a-zA-Z0-9-._]+)\/)?(?P<name>[a-zA-Z0-9-._]+)(?::(?P<tag>[a-zA-Z0-9-._]+))?$`
	re := regexp.MustCompile(pattern)

	// Match the input image URL against the pattern.
	matches := re.FindStringSubmatch(tag)
	if matches == nil {
		return nil, fmt.Errorf("invalid Docker image URL")
	}

	// Extract the captured groups into a map.
	groupNames := re.SubexpNames()
	result := make(map[string]string)
	for i, name := range groupNames {
		if i != 0 && name != "" {
			result[name] = matches[i]
		}
	}
	image := spec.Tag{
		Host:      result["host"],
		Namespace: result["namespace"],
		Name:      result["name"],
		Version:   result["tag"],
	}

	if image.Version == "" {
		image.Version = "latest"
	}

	return &image, nil
}

func UpdateTag( oldTag, newTag string) error {
	var index ModuleMap
	indexPath := paths.GetModuleMapPath()
	if FileExists(indexPath) {
		indexBytes, err := os.ReadFile(indexPath); if err != nil {
			return err
		}
		err = json.Unmarshal(indexBytes, &index); if err != nil {
			return err
		}

		if sha, ok := index.Modules[oldTag]; ok {
			index.Modules[newTag] = sha
			delete(index.Modules, oldTag)
			indexBytes, err := json.Marshal(index); if err != nil {
				return err
			}
			os.WriteFile(indexPath, indexBytes, 0644)
			fmt.Printf("Tag %s updated to %s\n", oldTag, newTag)
			return nil
		} else {
			return fmt.Errorf("tag not found in index")
		}
	} else {
		return fmt.Errorf("index file not found")
	}
}