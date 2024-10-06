package deployment

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/eunanio/nori/internal/backend"
	"github.com/eunanio/nori/internal/config"
	"github.com/eunanio/nori/internal/console"
	"github.com/eunanio/nori/internal/futils"
	"github.com/eunanio/nori/internal/hcl"
	"github.com/eunanio/nori/internal/paths"
	"github.com/eunanio/nori/internal/pull"
	"github.com/eunanio/nori/internal/spec"
	"github.com/eunanio/nori/internal/tf"
)

const (
	TYPE_PLAN  = 0
	TYPE_APPLY = 1
)

type DeploymentOpts struct {
	Tag          *spec.Tag
	ValuesPath   string
	ApplyType    int
	ReleaseId    string
	ProviderFile string
}

func Run(opts DeploymentOpts) error {
	currentConfig := config.Load()
	projectId := currentConfig.Project
	console.Debug("Project ID: " + projectId)
	//tmpDir := fmt.Sprintf("./%s", opts.ReleaseId)
	tmpDir := paths.GetReleasePath(opts.ReleaseId)
	paths.MkDirIfNotExist(tmpDir)
	if !futils.IsDebug() {
		defer cleanup(tmpDir)
	}

	_, config, err := pull.PullImage(opts.Tag, true, tmpDir)
	if err != nil {
		return err
	}

	if opts.ValuesPath == "" {
		return fmt.Errorf("values file required to plan deployments")
	}

	values, err := futils.ParseValuesFile(opts.ValuesPath, config)
	if err != nil {
		return err
	}

	//out
	console.Println("Generating Workspace...")

	hcl.GenerateModuleBlock(opts.Tag.Name, tmpDir, values)
	err = backend.GenerateBackendBlock(opts.ReleaseId)
	if err != nil {
		return err
	}
	if len(config.Outputs) > 0 {
		hcl.GenerateOutputsBlock(opts.Tag.Name, tmpDir, config.Outputs)
	}

	err = copyProviderFile(tmpDir, opts.ProviderFile)
	if err != nil {
		return err
	}

	switch opts.ApplyType {
	case TYPE_PLAN:
		plan(tmpDir)
	case TYPE_APPLY:
		apply(tmpDir)
	}
	if opts.ApplyType == TYPE_APPLY {
		console.Success("Release ID: " + opts.ReleaseId)
		valuesBytes, err := json.Marshal(values)
		if err != nil {
			return err
		}

		release := Release{
			Id:        opts.ReleaseId,
			Tag:       opts.Tag.String(),
			Values:    hex.EncodeToString(valuesBytes),
			Project:   projectId,
			UpdatedAt: time.Now(),
		}
		err = UpdateOrCreateReleaseState(release)
		if err != nil {
			return err
		}
	}

	return nil
}

func Destory(releaseId string) error {
	tmpDir := paths.GetReleasePath(releaseId)
	paths.MkDirIfNotExist(tmpDir)
	if !futils.IsDebug() {
		defer cleanup(tmpDir)
	}

	err := backend.GenerateBackendBlock(releaseId)
	if err != nil {
		console.Error("Error generating backend block: " + err.Error())
		return err
	}
	err = tf.Destroy(tmpDir)
	if err != nil {
		return err
	}

	err = RemoveReleaseFromState(releaseId)
	if err != nil {
		return err
	}

	return nil
}

func apply(path string) {
	console.Println("Applying...")
	err := tf.Apply(path)
	if err != nil {
		console.Error("Runtime error occured: " + err.Error())
		return
	}
}

func plan(path string) {
	console.Println("Planning...")
	err := tf.Plan(path)
	if err != nil {
		console.Error("Runtime error occured: " + err.Error())
		return
	}
}

func copyProviderFile(tmpDir, providerPath string) error {
	if providerPath != "" {
		if !futils.FileExists(providerPath) {
			return fmt.Errorf("provider file not found")
		}

		providerFile := fmt.Sprintf("%s/provider.tf", tmpDir)
		fileBytes, err := os.ReadFile(providerPath)
		if err != nil {
			return fmt.Errorf("Error reading provider file: " + err.Error())
		}
		err = os.WriteFile(providerFile, fileBytes, 0644)
		if err != nil {
			return fmt.Errorf("Error writing provider file: " + err.Error())
		}
	}

	return nil
}

func cleanup(path string) {
	console.Success("Cleaning up workspace...")
	os.RemoveAll(path)
	console.Println("Done")
}
