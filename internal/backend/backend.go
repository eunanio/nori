package backend

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"

	"github.com/eunanio/nori/internal/config"
	"github.com/eunanio/nori/internal/futils"
	"github.com/eunanio/nori/internal/paths"
)

type TFBlock struct {
	TFBackend TFBackend `json:"terraform"`
}

type TFBackend struct {
	Backend map[string]interface{} `json:"backend"`
}

func GenerateBackendBlock(releaseId string) error {
	path := paths.GetStatePath(releaseId)
	releasePath := paths.GetReleasePath(releaseId)
	config := config.Load()
	var backend *TFBlock
	if config.Remote != nil {
		if config.Region == nil {
			region := "eu-west-1"
			config.Region = &region
		}
		bucket_url, err := url.Parse(*config.Remote)
		if err != nil {
			return err
		}

		backend = s3Backend(releaseId, bucket_url.Host, *config.Region)
	} else {
		backend = localBackend(path)
	}

	jsonBytes, err := json.Marshal(backend)
	if err != nil {
		return err
	}

	err = os.WriteFile(fmt.Sprintf("%s/backend.tf.json", releasePath), jsonBytes, 0644)
	if err != nil {
		return err
	}

	return nil
}

// func GetBackend() ([]string, error) {
// 	config := config.Load()
// 	releaseUuid, err := uuid.NewV7()
// 	if err != nil {
// 		return nil, err
// 	}

// 	if config.Project == "" {
// 		return nil, fmt.Errorf("project not set in config, run `nori init`")
// 	}
// 	deplymentName := fmt.Sprintf("%s/%s", config.Project, releaseUuid.String())
// 	if futils.IsDebug() {
// 		fmt.Println("Deployment Name: ", deplymentName)
// 	}
// 	if config.Remote != nil {
// 		if config.Region == nil {
// 			region := "us-east-1"
// 			config.Region = &region
// 		}
// 		return s3BackendArgs(deplymentName, *config.Remote, *config.Region)
// 	} else {
// 		return localBackendArgs(deplymentName)
// 	}
// }

func s3Backend(name, bucket, region string) *TFBlock {
	keyPath := fmt.Sprintf("state/%s/terraform.tfstate", name)
	if futils.IsDebug() {
		fmt.Println("Key S3 Path: ", keyPath)
	}
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

// Not sure if going to support this
// func s3BackendArgs(name, bucket, region string) ([]string, error) {
// 	keyPath := fmt.Sprintf("releases/%s/terraform.tfstate", name)
// 	args := []string{
// 		"-backend-config=schema=s3",
// 		"-backend-config=bucket=" + bucket,
// 		"-backend-config=key=" + keyPath,
// 		"-backend-config=region=" + region,
// 	}

// 	return args, nil
// }

// func localBackendArgs(name string) ([]string, error) {
// 	path := paths.GetStatePath(name)
// 	args := []string{
// 		"-backend-config=schema=local",
// 		"-backend-config=path=" + fmt.Sprintf("%s/terraform.tfstate", path),
// 	}

// 	return args, nil
// }

func localBackend(path string) *TFBlock {
	if futils.IsDebug() {
		fmt.Println("Key Local Path: ", path)
	}
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
