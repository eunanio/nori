package commands

import (
	"fmt"

	nori "github.com/eunanio/nori/lib"
	"github.com/spf13/cobra"
)

type deployOptions struct {
	valuesFile    string
	values        []string
	workDir       string
	autoApprove   bool
	parallelism   int
	varFiles      []string
	backendConfig []string
	targets       []string
	destroy       bool
	planOnly      bool
	upgrade       bool
}

func newDeployCommand() *cobra.Command {
	opts := &deployOptions{}

	cmd := &cobra.Command{
		Use:   "deploy <reference>",
		Short: "Deploy a module from an OCI artifact",
		Long: `Deploy a OpenTofu module from an OCI artifact.

This command:
1. Pulls the module from the registry
2. Extracts it to a working directory
3. Applies values from values.yaml or --set flags
4. Runs tofu init and apply

Examples:
  # Deploy with a values file
  nori deploy ghcr.io/myorg/s3-bucket:v1.0.0 -f values.yaml

  # Deploy with inline values
  nori deploy ghcr.io/myorg/s3-bucket:v1.0.0 \
    --set bucket_name=my-bucket \
    --set versioning_enabled=true

  # Plan only (don't apply)
  nori deploy ghcr.io/myorg/s3-bucket:v1.0.0 -f values.yaml --plan-only

  # Deploy with auto-approve
  nori deploy ghcr.io/myorg/s3-bucket:v1.0.0 -f values.yaml --auto-approve

  # Destroy deployed infrastructure
  nori deploy ghcr.io/myorg/s3-bucket:v1.0.0 -f values.yaml --destroy`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDeploy(cmd, args, opts)
		},
	}

	cmd.Flags().StringVarP(&opts.valuesFile, "values", "f", "", "Path to values.yaml file")
	cmd.Flags().StringArrayVar(&opts.values, "set", nil, "Set values on the command line (key=value)")
	cmd.Flags().StringVar(&opts.workDir, "work-dir", "", "Working directory for deployment")
	cmd.Flags().BoolVar(&opts.autoApprove, "auto-approve", false, "Automatically approve apply")
	cmd.Flags().IntVar(&opts.parallelism, "parallelism", 10, "Number of parallel operations")
	cmd.Flags().StringArrayVar(&opts.varFiles, "var-file", nil, "Additional var files")
	cmd.Flags().StringArrayVar(&opts.backendConfig, "backend-config", nil, "Backend configuration (key=value)")
	cmd.Flags().StringArrayVar(&opts.targets, "target", nil, "Specific resources to target")
	cmd.Flags().BoolVar(&opts.destroy, "destroy", false, "Destroy infrastructure instead of creating")
	cmd.Flags().BoolVar(&opts.planOnly, "plan-only", false, "Only create plan, don't apply")
	cmd.Flags().BoolVar(&opts.upgrade, "upgrade", false, "Upgrade providers during init")

	return cmd
}

func runDeploy(cmd *cobra.Command, args []string, opts *deployOptions) error {
	reference := args[0]

	inlineValues, err := nori.ParseSetValues(opts.values)
	if err != nil {
		return err
	}

	backendConfig, err := nori.ParseAnnotations(opts.backendConfig)
	if err != nil {
		return fmt.Errorf("invalid backend config: %w", err)
	}

	result, err := getLibClient().Deploy(cmd.Context(), reference, nori.DeployOptions{
		ValuesFile:    opts.valuesFile,
		Values:        inlineValues,
		WorkDir:       opts.workDir,
		AutoApprove:   opts.autoApprove,
		Parallelism:   opts.parallelism,
		VarFiles:      opts.varFiles,
		BackendConfig: backendConfig,
		Targets:       opts.targets,
		Destroy:       opts.destroy,
		PlanOnly:      opts.planOnly,
		Upgrade:       opts.upgrade,
	})
	if err != nil {
		return err
	}

	fmt.Printf("\n✓ Deployment completed\n")
	fmt.Printf("  Module:   %s\n", result.ModuleRef)
	fmt.Printf("  WorkDir:  %s\n", result.WorkDir)

	if opts.planOnly {
		fmt.Printf("  Plan:     %s\n", result.PlanFile)
		fmt.Printf("\nTo apply this plan, run:\n")
		fmt.Printf("  cd %s/module && tofu apply %s\n", result.WorkDir, result.PlanFile)
	} else if result.Applied {
		fmt.Printf("  Status:   Applied\n")
		if len(result.Outputs) > 0 {
			fmt.Printf("  Outputs:\n")
			for k, v := range result.Outputs {
				fmt.Printf("    %s: %v\n", k, v)
			}
		}
	}

	return nil
}

func newDestroyCommand() *cobra.Command {
	opts := &deployOptions{}

	cmd := &cobra.Command{
		Use:   "destroy <work-dir>",
		Short: "Destroy deployed infrastructure",
		Long: `Destroy infrastructure that was previously deployed.

This command runs tofu destroy in the specified working directory.

Examples:
  # Destroy with auto-approve
  nori destroy /tmp/nori-deploy-xxx --auto-approve

  # Destroy specific targets
  nori destroy /tmp/nori-deploy-xxx --target aws_s3_bucket.main`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDestroy(cmd, args, opts)
		},
	}

	cmd.Flags().BoolVar(&opts.autoApprove, "auto-approve", false, "Automatically approve destroy")
	cmd.Flags().IntVar(&opts.parallelism, "parallelism", 10, "Number of parallel operations")
	cmd.Flags().StringArrayVar(&opts.targets, "target", nil, "Specific resources to target")

	return cmd
}

func runDestroy(cmd *cobra.Command, args []string, opts *deployOptions) error {
	workDir := args[0]

	if err := getLibClient().Destroy(cmd.Context(), workDir, nori.DestroyOptions{
		AutoApprove: opts.autoApprove,
		Parallelism: opts.parallelism,
		Targets:     opts.targets,
	}); err != nil {
		return err
	}

	fmt.Printf("✓ Infrastructure destroyed\n")
	return nil
}
