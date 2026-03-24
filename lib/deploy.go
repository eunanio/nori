package nori

import (
	"context"
	"fmt"

	"github.com/eunanio/nori/pkg/deploy"
)

// Deploy runs a one-off module deployment (without OCI-backed release state).
func (c *Client) Deploy(ctx context.Context, reference string, opts DeployOptions) (*DeployResult, error) {
	log := c.logger
	log.Info("deploying module", "reference", reference)

	backendConfig := opts.BackendConfig
	if backendConfig == nil {
		backendConfig = make(map[string]string)
	}

	deployer := deploy.NewDeployer(c.ociClient, "tofu", log)

	result, err := deployer.Deploy(ctx, reference, deploy.DeployOptions{
		ValuesFile:    opts.ValuesFile,
		Values:        opts.Values,
		WorkDir:       opts.WorkDir,
		AutoApprove:   opts.AutoApprove,
		Parallelism:   opts.Parallelism,
		VarFiles:      opts.VarFiles,
		BackendConfig: backendConfig,
		Targets:       opts.Targets,
		Destroy:       opts.Destroy,
		PlanOnly:      opts.PlanOnly,
		Upgrade:       opts.Upgrade,
	})
	if err != nil {
		return nil, fmt.Errorf("deployment failed: %w", err)
	}

	return &DeployResult{
		ModuleRef:  result.ModuleRef,
		WorkDir:    result.WorkDir,
		PlanFile:   result.PlanFile,
		HasChanges: result.HasChanges,
		Applied:    result.Applied,
		Outputs:    result.Outputs,
	}, nil
}

// Destroy destroys infrastructure in an existing working directory.
func (c *Client) Destroy(ctx context.Context, workDir string, opts DestroyOptions) error {
	log := c.logger
	log.Info("destroying infrastructure", "workDir", workDir)

	deployer := deploy.NewDeployer(c.ociClient, "tofu", log)

	return deployer.Destroy(ctx, workDir, deploy.DeployOptions{
		AutoApprove: opts.AutoApprove,
		Parallelism: opts.Parallelism,
		Targets:     opts.Targets,
	})
}
