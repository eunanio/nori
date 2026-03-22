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

type upgradeOptions struct {
	valuesFile        string
	values            []string
	tag               string
	annotations       []string
	autoApprove       bool
	parallelism       int
	varFiles          []string
	backendConfig     []string
	targets           []string
	planOnly          bool
	upgradeInit       bool
	reuseValues       bool
	resetValues       bool
	description       string
	version           string
	rollbackOnFailure bool
}

func newReleaseUpgradeCommand() *cobra.Command {
	opts := &upgradeOptions{}

	cmd := &cobra.Command{
		Use:   "upgrade <release_name> [module-reference]",
		Short: "Upgrade an existing release or check for drift",
		Long: `Upgrade an existing release with new values or a new module version.

This command updates an existing release. You can:
- Update values only (keeping the same module version)
- Update to a new module version using -t flag or positional argument
- Combine both
- Add or update annotations
- Run a drift check (no flags) to detect infrastructure drift or upstream module updates

When run without -t or -f flags, the command acts as a drift check:
- Pulls the existing state and module fresh from OCI
- Detects any infrastructure drift or upstream module changes
- Only increments version and pushes state if changes are applied

The release state is pulled from OCI, updated, and pushed back after successful deployment.

Examples:
  # Drift check - detect infrastructure drift or upstream module updates
  nori release upgrade my-bucket

  # Drift check with auto-approve to automatically correct drift
  nori release upgrade my-bucket --auto-approve

  # Upgrade with new values (same module version)
  nori release upgrade my-bucket -f values.yaml

  # Upgrade to a new module version using -t flag
  nori release upgrade my-bucket -t v2.0.0 -f values.yaml

  # Upgrade to a new module version (full reference)
  nori release upgrade my-bucket ghcr.io/myorg/s3-bucket:v2.0.0

  # Upgrade with inline values
  nori release upgrade my-bucket --set bucket_name=new-bucket

  # Upgrade and add annotations
  nori release upgrade my-bucket -f values.yaml --annotation release-notes="Fixed bug"

  # Upgrade and reuse previous values (merge with new)
  nori release upgrade my-bucket -f values.yaml --reuse-values

  # Upgrade and reset to default values
  nori release upgrade my-bucket -f values.yaml --reset-values`,
		Args: cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUpgrade(cmd, args, opts)
		},
	}

	cmd.Flags().StringVarP(&opts.valuesFile, "values", "f", "", "Path to values.yaml file")
	cmd.Flags().StringArrayVar(&opts.values, "set", nil, "Set values on the command line (key=value)")
	cmd.Flags().StringVarP(&opts.tag, "tag", "t", "", "New module tag/version to upgrade to")
	cmd.Flags().StringArrayVar(&opts.annotations, "annotation", nil, "Add or update annotations (key=value)")
	cmd.Flags().BoolVar(&opts.autoApprove, "auto-approve", false, "Automatically approve apply")
	cmd.Flags().IntVar(&opts.parallelism, "parallelism", 10, "Number of parallel operations")
	cmd.Flags().StringArrayVar(&opts.varFiles, "var-file", nil, "Additional var files")
	cmd.Flags().StringArrayVar(&opts.backendConfig, "backend-config", nil, "Backend configuration (key=value)")
	cmd.Flags().StringArrayVar(&opts.targets, "target", nil, "Specific resources to target")
	cmd.Flags().BoolVar(&opts.planOnly, "plan-only", false, "Only create plan, don't apply")
	cmd.Flags().BoolVar(&opts.upgradeInit, "upgrade", false, "Upgrade providers during init")
	cmd.Flags().BoolVar(&opts.reuseValues, "reuse-values", false, "Reuse the last release's values and merge with new ones")
	cmd.Flags().BoolVar(&opts.resetValues, "reset-values", false, "Reset values to the defaults")
	cmd.Flags().StringVar(&opts.description, "description", "", "Description for this release version")
	cmd.Flags().StringVar(&opts.version, "version", "", "Override the release version (default: auto-increment)")
	cmd.Flags().BoolVar(&opts.rollbackOnFailure, "rollback", false, "Rollback to previous state on failure")
	cmd.Flags().BoolVar(&opts.rollbackOnFailure, "rof", false, "Alias for --rollback")

	return cmd
}

func runUpgrade(cmd *cobra.Command, args []string, opts *upgradeOptions) error {
	ctx := cmd.Context()
	releaseName := args[0]

	// Optional new module reference (positional or via -t flag)
	var newModuleRef string
	if len(args) > 1 {
		newModuleRef = args[1]
	}

	// Detect drift check mode - no value or module changes provided
	// This mode checks for infrastructure drift and upstream module updates
	isDriftCheck := len(args) == 1 && // no positional module ref
		opts.tag == "" && // no -t flag
		opts.valuesFile == "" && // no -f flag
		len(opts.values) == 0 && // no --set values
		!opts.resetValues // not resetting values

	log := getLogger()
	cfg := getConfig()

	// Get state repository
	stateRepo, err := cfg.GetStateRepository()
	if err != nil {
		return fmt.Errorf("state repository not configured: %w\nRun: nori config set state_repository <oci-repo>", err)
	}

	// Create state store
	stateStore := state.NewStateStore(getClient(), log)

	// Get the latest version of the release from OCI
	latestVersion, err := stateStore.GetLatestVersion(ctx, stateRepo, releaseName)
	if err != nil {
		return fmt.Errorf("release %q not found in state repository: %w", releaseName, err)
	}

	// Pull existing state (flat format: repo:releaseName-version)
	stateRef := fmt.Sprintf("%s:%s", stateRepo, state.FormatReleaseTag(releaseName, latestVersion))
	existingState, err := stateStore.PullState(ctx, stateRef)
	if err != nil {
		return fmt.Errorf("failed to pull release state: %w", err)
	}

	if isDriftCheck {
		log.Info("running drift check",
			"name", releaseName,
			"version", existingState.Metadata.Version,
			"module", existingState.Metadata.ModuleRef,
		)
	} else {
		log.Info("upgrading release",
			"name", releaseName,
			"from_version", existingState.Metadata.Version,
			"module", existingState.Metadata.ModuleRef,
		)
	}

	// Determine if module is changing
	moduleChanged := false
	moduleRef := existingState.Metadata.ModuleRef

	if newModuleRef != "" {
		moduleRef = newModuleRef
		moduleChanged = true
	} else if opts.tag != "" {
		// Update the tag on the existing module reference
		// Extract base reference and replace tag
		moduleRef = updateModuleTag(existingState.Metadata.ModuleRef, opts.tag)
		moduleChanged = moduleRef != existingState.Metadata.ModuleRef
	}

	// Determine new version
	// In drift check mode, we defer version increment until we know changes will be applied
	var newVersion string
	if opts.version != "" {
		newVersion = state.EnsureVPrefix(opts.version)
	} else if isDriftCheck {
		// In drift check mode, initially use existing version
		// Will be updated to new version only if changes are applied
		newVersion = existingState.Metadata.Version
	} else {
		newVersion, err = state.NextVersion(existingState.Metadata.Version, moduleChanged)
		if err != nil {
			return fmt.Errorf("failed to calculate next version: %w", err)
		}
	}

	// Determine base values
	var baseValues map[string]interface{}
	if opts.resetValues {
		baseValues = make(map[string]interface{})
	} else if opts.reuseValues || opts.valuesFile == "" {
		// Parse existing values
		if len(existingState.Values) > 0 {
			if err := yaml.Unmarshal(existingState.Values, &baseValues); err != nil {
				log.Warn("failed to parse existing values", "error", err)
				baseValues = make(map[string]interface{})
			}
		} else {
			baseValues = make(map[string]interface{})
		}
	} else {
		baseValues = make(map[string]interface{})
	}

	// Load values from file
	var valuesYAML []byte
	if opts.valuesFile != "" {
		data, err := os.ReadFile(opts.valuesFile)
		if err != nil {
			return fmt.Errorf("failed to read values file: %w", err)
		}
		fileValues := make(map[string]interface{})
		if err := yaml.Unmarshal(data, &fileValues); err != nil {
			return fmt.Errorf("failed to parse values file: %w", err)
		}
		for k, v := range fileValues {
			baseValues[k] = v
		}
		valuesYAML = data
	}

	// Parse inline values (highest precedence)
	for _, v := range opts.values {
		key, value, err := parseValue(v)
		if err != nil {
			return err
		}
		baseValues[key] = value
	}

	// Re-marshal values if we have inline values
	if len(opts.values) > 0 || (opts.reuseValues && opts.valuesFile != "") {
		valuesYAML, _ = yaml.Marshal(baseValues)
	} else if valuesYAML == nil && len(existingState.Values) > 0 {
		valuesYAML = existingState.Values
	}

	// Parse annotations
	annotations := make(map[string]string)
	// Copy existing annotations
	for k, v := range existingState.Metadata.Annotations {
		annotations[k] = v
	}
	// Add/update new annotations
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
	rel.Values = baseValues
	rel.ValuesFile = opts.valuesFile
	rel.BackendConfig = backendConfig
	rel.Annotations = annotations
	rel.SemVer = newVersion

	// Create local release store
	localStore := release.NewStore("")

	// Write existing terraform state if we have it
	if len(existingState.TFState) > 0 {
		moduleDir := localStore.GetModuleDir(releaseName)
		if err := os.MkdirAll(moduleDir, 0755); err != nil {
			return fmt.Errorf("failed to create module directory: %w", err)
		}
		tfStatePath := filepath.Join(moduleDir, "terraform.tfstate")
		if err := os.WriteFile(tfStatePath, existingState.TFState, 0644); err != nil {
			log.Warn("failed to restore terraform state", "error", err)
		}
	}

	// Save release locally
	if err := localStore.Save(rel); err != nil {
		return fmt.Errorf("failed to save release: %w", err)
	}

	// Generate main.tf using codegen
	mainTF := codegen.GenerateMainTF(
		&codegen.ModuleConfig{
			Name:   releaseName,
			Source: moduleRef,
			Values: baseValues,
		},
		nil,
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

	// Deploy (upgrade)
	result, err := deployer.DeployRelease(ctx, rel, localStore, deploy.ReleaseDeployOptions{
		AutoApprove: opts.autoApprove,
		Parallelism: opts.parallelism,
		VarFiles:    opts.varFiles,
		Targets:     opts.targets,
		PlanOnly:    opts.planOnly,
		Upgrade:     opts.upgradeInit,
		Reconfigure: true,
	})

	if err != nil {
		rel.Status = release.StatusFailed
		localStore.Save(rel)

		if opts.rollbackOnFailure {
			prevVersion, findErr := stateStore.GetLastSuccessfulVersion(ctx, stateRepo, releaseName)
			if findErr != nil {
				log.Warn("failed to find previous successful version for rollback", "error", findErr)
			} else if prevVersion == "" {
				log.Warn("no previous successful version found for rollback")
			} else if prevVersion != latestVersion || existingState.Metadata.Status != state.StatusDeployed {
				log.Info("attempting rollback", "target_version", prevVersion)
				prevStateRef := fmt.Sprintf("%s:%s", stateRepo, state.FormatReleaseTag(releaseName, prevVersion))
				prevState, pullErr := stateStore.PullState(ctx, prevStateRef)
				if pullErr != nil {
					log.Warn("failed to pull previous state for rollback", "error", pullErr)
				} else {
					rollbackResult, rollbackErr := deployer.RollbackRelease(ctx, releaseName, prevState, localStore, deploy.ReleaseDeployOptions{
						AutoApprove: true,
						Parallelism: opts.parallelism,
						Reconfigure: true,
					})
					if rollbackErr != nil {
						log.Error("rollback failed", "error", rollbackErr)
						fmt.Printf("\nERROR: Rollback to %s failed: %v\n", prevVersion, rollbackErr)
					} else {
						fmt.Printf("\nROLLBACK: Successfully restored to version %s\n", prevVersion)
						if rollbackResult != nil && len(rollbackResult.TFState) > 0 {
							result = rollbackResult
						}
					}
				}
			} else {
				log.Debug("skipping rollback, already at last successful version")
			}
		}

		if result != nil && result.ApplyAttempted && len(result.TFState) > 0 {
			newStateRef, pushErr := pushReleaseState(ctx, stateStore, stateRepo, PushStateParams{
				ReleaseName: releaseName,
				ModuleRef:   moduleRef,
				Version:     newVersion,
				Status:      state.StatusFailed,
				MainTF:      mainTF,
				TFState:     result.TFState,
				Values:      valuesYAML,
				Description: opts.description,
				Annotations: annotations,
			}, log)
			if pushErr != nil {
				log.Warn("failed to push failed release state", "error", pushErr)
			} else {
				rel.StateRef = newStateRef
				localStore.Save(rel)
				fmt.Printf("\nNOTE: Failed release state pushed to OCI for recovery: %s\n", newStateRef)
			}
		}

		return fmt.Errorf("upgrade failed: %w", err)
	}

	// Update release status
	if result.Applied {
		rel.Status = release.StatusDeployed
	}
	if err := localStore.Save(rel); err != nil {
		log.Warn("failed to update release status", "error", err)
	}

	// Push state to OCI if deployment was successful and changes were applied
	if !opts.planOnly && result.Applied {
		// In drift check mode, calculate new version now that we know changes were applied
		if isDriftCheck && opts.version == "" {
			newVersion, err = state.NextVersion(existingState.Metadata.Version, false)
			if err != nil {
				log.Warn("failed to calculate next version for drift correction", "error", err)
				// Fall back to patch increment
				newVersion = existingState.Metadata.Version
			}
			rel.SemVer = newVersion
		}

		newStateRef, pushErr := pushReleaseState(ctx, stateStore, stateRepo, PushStateParams{
			ReleaseName: releaseName,
			ModuleRef:   moduleRef,
			Version:     newVersion,
			Status:      state.StatusDeployed,
			MainTF:      mainTF,
			TFState:     result.TFState,
			Values:      valuesYAML,
			Description: opts.description,
			Annotations: annotations,
		}, log)
		if pushErr != nil {
			log.Warn("failed to push release state", "error", pushErr)
			fmt.Printf("\nWARNING: Release upgraded but state push failed: %v\n", pushErr)
		} else {
			rel.StateRef = newStateRef
			localStore.Save(rel)
		}
	}

	// Print result
	fmt.Printf("\n")
	if opts.planOnly {
		fmt.Printf("NAME: %s\n", rel.Name)
		if isDriftCheck {
			fmt.Printf("STATUS: drift check (plan only)\n")
		} else {
			fmt.Printf("STATUS: planned\n")
		}
		if isDriftCheck || !result.HasChanges {
			fmt.Printf("VERSION: %s\n", existingState.Metadata.Version)
		} else {
			fmt.Printf("VERSION: %s -> %s\n", existingState.Metadata.Version, newVersion)
		}
		fmt.Printf("MODULE: %s\n", rel.ModuleRef)
		fmt.Printf("PLAN: %s\n", result.PlanFile)
		if !result.HasChanges {
			if isDriftCheck {
				fmt.Printf("\nDrift check complete. Infrastructure is in sync.\n")
			} else {
				fmt.Printf("\nNo changes detected. Infrastructure is up-to-date.\n")
			}
		} else {
			if isDriftCheck {
				fmt.Printf("\nDrift detected. To correct drift, run:\n")
			} else {
				fmt.Printf("\nTo apply this plan, run:\n")
			}
			fmt.Printf("  nori release upgrade %s --auto-approve\n", releaseName)
		}
	} else if !result.HasChanges {
		fmt.Printf("NAME: %s\n", rel.Name)
		if isDriftCheck {
			fmt.Printf("STATUS: synced\n")
		} else {
			fmt.Printf("STATUS: no changes\n")
		}
		fmt.Printf("VERSION: %s\n", existingState.Metadata.Version)
		fmt.Printf("MODULE: %s\n", rel.ModuleRef)
		if isDriftCheck {
			fmt.Printf("\nDrift check complete. Infrastructure is in sync.\n")
		} else {
			fmt.Printf("\nNo changes detected. Infrastructure is up-to-date.\n")
		}
	} else {
		fmt.Printf("NAME: %s\n", rel.Name)
		if isDriftCheck {
			fmt.Printf("STATUS: drift corrected\n")
		} else {
			fmt.Printf("STATUS: %s\n", rel.Status)
		}
		fmt.Printf("VERSION: %s -> %s\n", existingState.Metadata.Version, newVersion)
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

		if isDriftCheck {
			fmt.Printf("\nDrift corrected. Infrastructure is now in sync.\n")
		}
	}

	return nil
}

// updateModuleTag updates the tag portion of a module reference.
// Example: "ghcr.io/org/module:v1.0.0" with tag "v2.0.0" -> "ghcr.io/org/module:v2.0.0"
func updateModuleTag(moduleRef, newTag string) string {
	// Find the last colon that's part of the tag (not the port)
	lastColon := -1
	for i := len(moduleRef) - 1; i >= 0; i-- {
		if moduleRef[i] == ':' {
			// Check if this looks like a tag (no slashes after it)
			hasSlash := false
			for j := i + 1; j < len(moduleRef); j++ {
				if moduleRef[j] == '/' {
					hasSlash = true
					break
				}
			}
			if !hasSlash {
				lastColon = i
				break
			}
		}
	}

	if lastColon == -1 {
		// No tag found, append the new tag
		return moduleRef + ":" + newTag
	}

	// Replace the existing tag
	return moduleRef[:lastColon+1] + newTag
}
