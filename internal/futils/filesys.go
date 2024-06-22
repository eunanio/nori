package futils

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/eunanhardy/nori/internal/paths"
	"github.com/eunanhardy/nori/internal/spec"
	"gopkg.in/yaml.v2"
)

type BlobWriter struct {
	ConfigPath string
	RepoName string
	RepoVersion string
	Data []byte
	Digest string
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

func LoadBlobContent(configPath string, digest string, tag *spec.Tag) ([]byte, error) {
	sha := strings.Split(digest, ":")[1]
	if sha == "" {
		return nil, fmt.Errorf("invalid digest")
	}

	filePath := paths.GetBlobPath(tag.Name, tag.Version, sha)
	if !FileExists(filePath) {
		return nil, fmt.Errorf("file does not exist")
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	
	return data, nil
}

func WriteBlobContent(opts BlobWriter) error {
	sha := strings.Split(opts.Digest, ":")[1]
	if sha == "" {
		return fmt.Errorf("invalid digest")
	}

	filePath := paths.GetBlobPath(opts.RepoName,opts.RepoVersion, sha)
	err := os.WriteFile(filePath, opts.Data, 0644)
	if err != nil {
		return err
	}
	
	return nil
}

func ParseValuesFile(file string, config *spec.Config) (values map[string]interface{}, err error) {

	fileBytes, err := os.ReadFile(file); if err != nil {
		return nil, fmt.Errorf("Error reading values file: "+ err.Error())
	}

	if filepath.Ext(file) == ".json" {
		err = json.Unmarshal(fileBytes, &values); if err != nil {
			return nil, err
		}
	} else if filepath.Ext(file) == ".yaml" || filepath.Ext(file) == ".yml" {
		err = yaml.Unmarshal(fileBytes, &values); if err != nil {
			return nil, err
		}
	} else {
		fmt.Println("Error: Unsupported values file type")
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

	return values, nil
}
