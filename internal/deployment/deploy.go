package deployment

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"

	"github.com/eunanhardy/nori/internal/backend"
	"github.com/eunanhardy/nori/internal/futils"
	"github.com/eunanhardy/nori/internal/hclparse"
	"github.com/eunanhardy/nori/internal/oci"
	"github.com/eunanhardy/nori/internal/paths"
	"github.com/eunanhardy/nori/internal/pull"
	"github.com/eunanhardy/nori/internal/spec"
	"github.com/eunanhardy/nori/internal/tf"
	"github.com/google/uuid"
)

const (
	TYPE_PLAN = 0
	TYPE_APPLY = 1
)

type DeploymentOpts struct {
	Tag *spec.Tag
	ValuesPath string
	ApplyType int
	ReleaseId string
}

func Run(opts DeploymentOpts) error {
	if opts.ReleaseId == "" {
		deploymentId, err := uuid.NewV7()
		if err != nil {
			slog.Error(err.Error())
			os.Exit(1)
		}
		opts.ReleaseId = deploymentId.String()
	}
	_, err := uuid.Parse(opts.ReleaseId)
	if err != nil {
		return fmt.Errorf("invalid ReleaseId, must be a uuid: %s", opts.ReleaseId)
	}

	tmpDir := fmt.Sprintf("./%s", opts.ReleaseId)
	paths.MkDirIfNotExist(tmpDir)

	var config spec.Config
	creds, err := oci.GetCredentials(opts.Tag.Host)
	if err != nil {
		return err
	}
	registry := oci.NewRegistry(opts.Tag.Host, creds)
	manifest, err := registry.PullManifest(opts.Tag)
	if err != nil {
		return err
	}

	configOpts := oci.PullBlobOptions{
		Digest: manifest.Config,
		Name:   opts.Tag.Name,
	}

	configBlob, err := registry.PullBlob(configOpts)
	if err != nil {
		return err
	}
	err = json.Unmarshal(configBlob, &config)
	if err != nil {
		return err
	}
	values, err := futils.ParseValuesFile(opts.ValuesPath, config)
	if err != nil {
		return err
	}

	pull.PullImage(opts.Tag, true,tmpDir)
	fmt.Println("Generating Workspace...")
	hclparse.GenerateModuleBlock(opts.Tag.Name, tmpDir, values)
	backend.GenerateBackendBlock(opts.ReleaseId)

	switch(opts.ApplyType) {
		case TYPE_PLAN:
			plan(tmpDir)
		case TYPE_APPLY:
			apply(tmpDir)
	}
	if opts.ApplyType == TYPE_APPLY {
		fmt.Println("Release ID: ", opts.ReleaseId)

		// Working on outputs

		// if len(config.Outputs) > 0 {
		// 	fmt.Println("--- Outputs ---")
		// 	tfOut, err := tf.Output(tmpDir)
		// 	if err != nil {
		// 		return fmt.Errorf("error getting outputs: %s", err.Error())
		// 	}
		// 	fmt.Println(tfOut)
		// }
	}

	cleanup(tmpDir)
	return nil
}


func apply(path string) {
	fmt.Println("Initiating Terraform Apply...")
	tfout, err := tf.Apply(path)
	if err != nil {
		slog.Error("Issue executiing runtime", err)
	}
	fmt.Println(tfout)
}

func plan(path string) {
	fmt.Println("Initiating Terraform Plan...")
	tfout, err := tf.Plan(path)
	if err != nil {
		slog.Error("Issue executiing runtime", err, tfout)
	}
	fmt.Println(tfout)
}

func cleanup(path string) {
	fmt.Println("Cleaning up workspace...")
	os.RemoveAll(path)
}