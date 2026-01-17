package commands

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/eunanio/nori/pkg/codegen"
	"github.com/eunanio/nori/pkg/deploy"
	"github.com/eunanio/nori/pkg/release"
	"github.com/eunanio/nori/pkg/state"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
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

This command creates a new release that can be upgraded, listed, and uninstalled.
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
	ctx := cmd.Context()
	releaseName := args[0]
	moduleRef := args[1]

	log := getLogger()
	cfg := getConfig()

	// Get state repository
	stateRepo, err := cfg.GetStateRepository()
	if err != nil {
		return fmt.Errorf("state repository not configured: %w\nRun: nori config set state_repository <oci-repo>", err)
	}

	// Create state store
	stateStore := state.NewStateStore(getClient(), log)

	// Check if release already exists in OCI state
	exists, _ := stateStore.ReleaseExists(ctx, stateRepo, releaseName)
	if exists {
		return fmt.Errorf("release %q already exists. Use 'nori upgrade' to update it", releaseName)
	}

	log.Info("creating release", "name", releaseName, "module", moduleRef)

	// Parse inline values
	inlineValues := make(map[string]interface{})
	for _, v := range opts.values {
		key, value, err := parseValue(v)
		if err != nil {
			return err
		}
		inlineValues[key] = value
	}

	// Load values from file
	values := make(map[string]interface{})
	var valuesYAML []byte
	if opts.valuesFile != "" {
		data, err := os.ReadFile(opts.valuesFile)
		if err != nil {
			return fmt.Errorf("failed to read values file: %w", err)
		}
		if err := yaml.Unmarshal(data, &values); err != nil {
			return fmt.Errorf("failed to parse values file: %w", err)
		}
		valuesYAML = data
	}

	// Merge inline values (they take precedence)
	for k, v := range inlineValues {
		values[k] = v
	}

	// If we have inline values, re-marshal to get the complete values YAML
	if len(inlineValues) > 0 {
		valuesYAML, _ = yaml.Marshal(values)
	}

	// Parse annotations
	annotations := make(map[string]string)
	for _, a := range opts.annotations {
		key, value, err := parseAnnotation(a)
		if err != nil {
			return fmt.Errorf("invalid annotation: %w", err)
		}
		annotations[key] = value
	}

	// Parse backend config
	backendConfig := make(map[string]string)
	for _, bc := range opts.backendConfig {
		key, value, err := parseAnnotation(bc)
		if err != nil {
			return fmt.Errorf("invalid backend config: %w", err)
		}
		backendConfig[key] = value
	}

	// Create local release for deployment
	rel := release.NewRelease(releaseName, moduleRef)
	rel.Values = values
	rel.ValuesFile = opts.valuesFile
	rel.BackendConfig = backendConfig
	rel.Annotations = annotations

	// Create local release store for deployment working directory
	localStore := release.NewStore("")

	// Save release locally as pending
	if err := localStore.Save(rel); err != nil {
		return fmt.Errorf("failed to save release: %w", err)
	}

	// Generate main.tf using codegen
	mainTF := codegen.GenerateMainTF(
		&codegen.ModuleConfig{
			Name:   releaseName,
			Source: moduleRef,
			Values: values,
		},
		nil, // backend config is handled separately
	)

	// Write main.tf to module directory before deployment
	moduleDir := localStore.GetModuleDir(releaseName)
	if err := os.MkdirAll(moduleDir, 0755); err != nil {
		return fmt.Errorf("failed to create module directory: %w", err)
	}
	mainTFPath := filepath.Join(moduleDir, "main.tf")
	if err := os.WriteFile(mainTFPath, mainTF, 0644); err != nil {
		return fmt.Errorf("failed to write main.tf: %w", err)
	}
	log.Debug("wrote main.tf", "path", mainTFPath)

	// Create deployer
	deployer := deploy.NewDeployer(getClient(), "tofu", log)

	// Deploy
	result, err := deployer.DeployRelease(ctx, rel, localStore, deploy.ReleaseDeployOptions{
		AutoApprove: opts.autoApprove,
		Parallelism: opts.parallelism,
		VarFiles:    opts.varFiles,
		Targets:     opts.targets,
		PlanOnly:    opts.planOnly,
		Upgrade:     opts.upgrade,
	})

	if err != nil {
		// Update release status to failed
		rel.Status = release.StatusFailed
		localStore.Save(rel)
		return fmt.Errorf("deployment failed: %w", err)
	}

	// Update release status
	if result.Applied {
		rel.Status = release.StatusDeployed
	}
	if err := localStore.Save(rel); err != nil {
		log.Warn("failed to update release status", "error", err)
	}

	// If not plan-only and deployment succeeded, push state to OCI
	if !opts.planOnly && result.Applied {
		log.Info("pushing release state to OCI", "repository", stateRepo)

		// Read terraform state file
		moduleDir := localStore.GetModuleDir(releaseName)
		tfStatePath := filepath.Join(moduleDir, "terraform.tfstate")
		tfState, err := os.ReadFile(tfStatePath)
		if err != nil {
			log.Warn("failed to read terraform state", "error", err)
			tfState = nil
		}

		// Create release state
		releaseState := &state.ReleaseState{
			Metadata: state.NewReleaseMetadata(releaseName, moduleRef, rel.SemVer),
			MainTF:   mainTF,
			TFState:  tfState,
			Values:   valuesYAML,
		}

		// Set metadata fields
		releaseState.Metadata.Status = state.StatusDeployed
		releaseState.Metadata.Description = opts.description
		for k, v := range annotations {
			releaseState.Metadata.SetAnnotation(k, v)
		}

		// Build state reference (flat format: repo:releaseName-version)
		stateRef := fmt.Sprintf("%s:%s", stateRepo, state.FormatReleaseTag(releaseName, rel.SemVer))

		// Push state to OCI
		if err := stateStore.PushState(ctx, stateRef, releaseState); err != nil {
			log.Warn("failed to push release state", "error", err)
			fmt.Printf("\nWARNING: Release deployed but state push failed: %v\n", err)
		} else {
			rel.StateRef = stateRef
			localStore.Save(rel)
		}
	}

	// Print result
	fmt.Printf("\n")
	if opts.planOnly {
		fmt.Printf("NAME: %s\n", rel.Name)
		fmt.Printf("STATUS: planned\n")
		fmt.Printf("VERSION: %s\n", rel.SemVer)
		fmt.Printf("PLAN: %s\n", result.PlanFile)
		if !result.HasChanges {
			fmt.Printf("\nNo changes detected. Infrastructure is up-to-date.\n")
		} else {
			fmt.Printf("\nTo apply this plan, run:\n")
			fmt.Printf("  nori upgrade %s --auto-approve\n", releaseName)
		}
	} else if !result.HasChanges {
		fmt.Printf("NAME: %s\n", rel.Name)
		fmt.Printf("STATUS: no changes\n")
		fmt.Printf("VERSION: %s\n", rel.SemVer)
		fmt.Printf("MODULE: %s\n", rel.ModuleRef)
		fmt.Printf("\nNo changes detected. Infrastructure is up-to-date.\n")
	} else {
		fmt.Printf("NAME: %s\n", rel.Name)
		fmt.Printf("STATUS: %s\n", rel.Status)
		fmt.Printf("VERSION: %s\n", rel.SemVer)
		fmt.Printf("MODULE: %s\n", rel.ModuleRef)

		if rel.StateRef != "" {
			fmt.Printf("STATE: %s\n", rel.StateRef)
		}

		if len(rel.Annotations) > 0 {
			fmt.Printf("\nANNOTATIONS:\n")
			for k, v := range rel.Annotations {
				fmt.Printf("  %s: %s\n", k, v)
			}
		}

		if len(result.Outputs) > 0 {
			fmt.Printf("\nOUTPUTS:\n")
			for k, v := range result.Outputs {
				fmt.Printf("  %s: %v\n", k, v)
			}
		}
	}

	return nil
}
