package commands

import (
	"fmt"

	nori "github.com/eunanio/nori/lib"
	"github.com/spf13/cobra"
)

type createOptions struct {
	valuesFile    string
	values        []string
	annotations   []string
	autoApprove   bool
	parallelism   int
	varFiles      []string
	backendConfig []string
	targets       []string
	planOnly      bool
	upgrade       bool
	description   string
}

func newReleaseCreateCommand() *cobra.Command {
	opts := &createOptions{}

	cmd := &cobra.Command{
		Use:     "create <release_name> <oci-tag>",
		Short:   "Create a new release from an OCI module",
		Aliases: []string{"apply", "install"},
		Long: `Create a new release by deploying a OpenTofu module from an OCI artifact.

This command creates a new release that can be upgraded, listed, and destroyed.
The release state is stored as an OCI artifact in the configured state repository.

Examples:
  # Create with a values file
  nori release create my-bucket ghcr.io/myorg/s3-bucket:v1.0.0 -f values.yaml

  # Create with inline values
  nori release create my-bucket ghcr.io/myorg/s3-bucket:v1.0.0 \
    --set bucket_name=my-bucket \
    --set versioning_enabled=true

  # Create with custom annotations
  nori release create my-bucket ghcr.io/myorg/s3-bucket:v1.0.0 -f values.yaml \
    --annotation team=platform \
    --annotation environment=production

  # Plan only (don't apply)
  nori release create my-bucket ghcr.io/myorg/s3-bucket:v1.0.0 -f values.yaml --plan-only

  # Create with auto-approve
  nori release create my-bucket ghcr.io/myorg/s3-bucket:v1.0.0 -f values.yaml --auto-approve`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCreate(cmd, args, opts)
		},
	}

	cmd.Flags().StringVarP(&opts.valuesFile, "values", "f", "", "Path to values.yaml file")
	cmd.Flags().StringArrayVar(&opts.values, "set", nil, "Set values on the command line (key=value)")
	cmd.Flags().StringArrayVar(&opts.annotations, "annotation", nil, "Add annotations to the release (key=value)")
	cmd.Flags().BoolVar(&opts.autoApprove, "auto-approve", false, "Automatically approve apply")
	cmd.Flags().IntVar(&opts.parallelism, "parallelism", 10, "Number of parallel operations")
	cmd.Flags().StringArrayVar(&opts.varFiles, "var-file", nil, "Additional var files")
	cmd.Flags().StringArrayVar(&opts.backendConfig, "backend-config", nil, "Backend configuration (key=value)")
	cmd.Flags().StringArrayVar(&opts.targets, "target", nil, "Specific resources to target")
	cmd.Flags().BoolVar(&opts.planOnly, "plan-only", false, "Only create plan, don't apply")
	cmd.Flags().BoolVar(&opts.upgrade, "upgrade", false, "Upgrade providers during init")
	cmd.Flags().StringVar(&opts.description, "description", "", "Description for this release")

	return cmd
}

func runCreate(cmd *cobra.Command, args []string, opts *createOptions) error {
	releaseName := args[0]
	moduleRef := args[1]

	inlineValues, err := nori.ParseSetValues(opts.values)
	if err != nil {
		return err
	}

	annotations, err := nori.ParseAnnotations(opts.annotations)
	if err != nil {
		return fmt.Errorf("invalid annotation: %w", err)
	}

	backendConfig, err := nori.ParseAnnotations(opts.backendConfig)
	if err != nil {
		return fmt.Errorf("invalid backend config: %w", err)
	}

	result, err := getLibClient().CreateRelease(cmd.Context(), releaseName, moduleRef, nori.CreateReleaseOptions{
		ValuesFile:    opts.valuesFile,
		Values:        inlineValues,
		Annotations:   annotations,
		AutoApprove:   opts.autoApprove,
		Parallelism:   opts.parallelism,
		VarFiles:      opts.varFiles,
		BackendConfig: backendConfig,
		Targets:       opts.targets,
		PlanOnly:      opts.planOnly,
		Upgrade:       opts.upgrade,
		Description:   opts.description,
	})
	if err != nil {
		return err
	}

	printReleaseResult(result)
	return nil
}

func printReleaseResult(r *nori.ReleaseResult) {
	fmt.Printf("\n")
	if r.PlanOnly {
		fmt.Printf("NAME: %s\n", r.Name)
		if r.IsDriftCheck {
			fmt.Printf("STATUS: drift check (plan only)\n")
		} else {
			fmt.Printf("STATUS: planned\n")
		}
		fmt.Printf("VERSION: %s\n", r.Version)
		if !r.HasChanges {
			if r.IsDriftCheck {
				fmt.Printf("\nDrift check complete. Infrastructure is in sync.\n")
			} else {
				fmt.Printf("\nNo changes detected. Infrastructure is up-to-date.\n")
			}
		} else {
			if r.IsDriftCheck {
				fmt.Printf("\nDrift detected. To correct drift, run:\n")
			} else {
				fmt.Printf("\nTo apply this plan, run:\n")
			}
			fmt.Printf("  nori release upgrade %s --auto-approve\n", r.Name)
		}
	} else if !r.HasChanges {
		fmt.Printf("NAME: %s\n", r.Name)
		if r.IsDriftCheck {
			fmt.Printf("STATUS: synced\n")
		} else {
			fmt.Printf("STATUS: no changes\n")
		}
		fmt.Printf("VERSION: %s\n", r.Version)
		fmt.Printf("MODULE: %s\n", r.ModuleRef)
		if r.IsDriftCheck {
			fmt.Printf("\nDrift check complete. Infrastructure is in sync.\n")
		} else {
			fmt.Printf("\nNo changes detected. Infrastructure is up-to-date.\n")
		}
	} else {
		fmt.Printf("NAME: %s\n", r.Name)
		if r.IsDriftCheck {
			fmt.Printf("STATUS: drift corrected\n")
		} else {
			fmt.Printf("STATUS: %s\n", r.Status)
		}
		fmt.Printf("VERSION: %s\n", r.Version)
		fmt.Printf("MODULE: %s\n", r.ModuleRef)

		if r.StateRef != "" {
			fmt.Printf("STATE: %s\n", r.StateRef)
		}

		if len(r.Annotations) > 0 {
			fmt.Printf("\nANNOTATIONS:\n")
			for k, v := range r.Annotations {
				fmt.Printf("  %s: %s\n", k, v)
			}
		}

		if len(r.Outputs) > 0 {
			fmt.Printf("\nOUTPUTS:\n")
			for k, v := range r.Outputs {
				fmt.Printf("  %s: %v\n", k, v)
			}
		}

		if r.IsDriftCheck {
			fmt.Printf("\nDrift corrected. Infrastructure is now in sync.\n")
		}
	}
}
