package deployment

import (
	"fmt"
	"log/slog"
	"os"
	"regexp"

	"github.com/eunanhardy/nori/internal/backend"
	"github.com/eunanhardy/nori/internal/config"
	"github.com/eunanhardy/nori/internal/futils"
	"github.com/eunanhardy/nori/internal/hcl"
	"github.com/eunanhardy/nori/internal/paths"
	"github.com/eunanhardy/nori/internal/pull"
	"github.com/eunanhardy/nori/internal/spec"
	"github.com/eunanhardy/nori/internal/tf"
	"github.com/google/uuid"
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
	if opts.ReleaseId == "" {
		deploymentId, err := uuid.NewV7()
		if err != nil {
			slog.Error(err.Error())
			os.Exit(1)
		}
		opts.ReleaseId = fmt.Sprintf("%s/%s", currentConfig.Project, deploymentId)
	}

	re := regexp.MustCompile(`([^\/]+)\/([0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12})$`)
	matches := re.FindStringSubmatch(opts.ReleaseId)
	if matches == nil {
		return fmt.Errorf("invalid release ID")
	}

	//tmpDir := fmt.Sprintf("./%s", opts.ReleaseId)
	tmpDir := paths.GetReleasePath(opts.ReleaseId)
	paths.MkDirIfNotExist(tmpDir)
	if !futils.IsDebug() {
		defer cleanup(tmpDir)
	}

	_, config := pull.PullImage(opts.Tag, true, tmpDir)
	values, err := futils.ParseValuesFile(opts.ValuesPath, config)
	if err != nil {
		return err
	}

	fmt.Println("Generating Workspace...")
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
		fmt.Println("Release ID: ", opts.ReleaseId)
	}

	return nil
}

func Destory(releaseId string) error {
	re := regexp.MustCompile(`([^\/]+)\/([0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12})$`)
	matches := re.FindStringSubmatch(releaseId)
	if matches == nil {
		return fmt.Errorf("invalid release ID")
	}

	tmpDir := paths.GetReleasePath(releaseId)
	paths.MkDirIfNotExist(tmpDir)
	if !futils.IsDebug() {
		defer cleanup(tmpDir)
	}

	err := backend.GenerateBackendBlock(releaseId)
	if err != nil {
		return err
	}
	tfout, err := tf.Destroy(tmpDir)
	if err != nil {
		return err
	}
	fmt.Println(tfout)

	return nil
}

func apply(path string) {
	fmt.Println("Initiating Terraform Apply...")
	tfout, err := tf.Apply(path)
	if err != nil {
		slog.Error("Runtime error occured", err.Error(), tfout)
	}
	fmt.Println(tfout)
}

func plan(path string) {
	fmt.Println("Initiating Terraform Plan...")
	tfout, err := tf.Plan(path)
	if err != nil {
		slog.Error("Runtime error occured", err.Error(), tfout)
	}
	fmt.Println(tfout)
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
	fmt.Println("Cleaning up workspace...")
	os.RemoveAll(path)
}
