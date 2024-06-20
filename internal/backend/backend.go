package backend

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"

	"github.com/eunanhardy/nori/internal/config"
	"github.com/eunanhardy/nori/internal/paths"
	"github.com/google/uuid"
)

type TFBlock struct {
	TFBackend TFBackend `json:"terraform"`
}

type TFBackend struct {
	Backend map[string]interface{} `json:"backend"`
}

func GenerateBackendBlock(deploymentId string) (error) {
	path := fmt.Sprintf("./%s", deploymentId)
	config := config.Load()
	var backend *TFBlock
	if config.Remote != nil {
		if config.Region == nil {
			region := "us-east-1"
			config.Region = &region
		}
		bucket_url, err := url.Parse(*config.Remote)
		if err != nil {
			return err
		}
		
		backend = s3Backend(deploymentId, bucket_url.Host, *config.Region)
	} else {
		backend = localBackend(deploymentId)
	}

	jsonBytes, err := json.Marshal(backend)
	if err != nil {
		return err
	}

	err = os.WriteFile(fmt.Sprintf("%s/backend.tf.json", path), jsonBytes, 0644)
	if err != nil {
		return err
	}


	return nil
}


func s3Backend(name, bucket,region string) *TFBlock {
	keyPath := fmt.Sprintf("releases/%s/terraform.tfstate", name)
	backend := &TFBlock{
		TFBackend: TFBackend{
			Backend: map[string]interface{}{
				"s3": map[string]interface{}{
					"bucket": bucket,
					"key":    keyPath,
					"region": region,
				},
			},
		},
	}

	return backend
}

func localBackend(name string) *TFBlock {
	path := paths.GetReleasePath(name)
	paths.MkDirIfNotExist(path)
	backend := &TFBlock{
		TFBackend: TFBackend{
			Backend: map[string]interface{}{
				"local": map[string]interface{}{
					"path": fmt.Sprintf("%s/terraform.tfstate", path),
				},
			},
		},
	}

	return backend
}

func CreateDeploymentId() (string,error) {
	uuid, err := uuid.NewV7()
	if err != nil {
		return "", err
	}
	return uuid.String(), nil
}