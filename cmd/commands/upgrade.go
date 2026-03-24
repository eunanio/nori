package commands

import (
	"fmt"

	nori "github.com/eunanio/nori/lib"
	"github.com/spf13/cobra"
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
	releaseName := args[0]

	var newModuleRef string
	if len(args) > 1 {
		newModuleRef = args[1]
	}

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

	result, err := getLibClient().UpgradeRelease(cmd.Context(), releaseName, nori.UpgradeReleaseOptions{
		ModuleRef:         newModuleRef,
		Tag:               opts.tag,
		ValuesFile:        opts.valuesFile,
		Values:            inlineValues,
		Annotations:       annotations,
		AutoApprove:       opts.autoApprove,
		Parallelism:       opts.parallelism,
		VarFiles:          opts.varFiles,
		BackendConfig:     backendConfig,
		Targets:           opts.targets,
		PlanOnly:          opts.planOnly,
		UpgradeInit:       opts.upgradeInit,
		ReuseValues:       opts.reuseValues,
		ResetValues:       opts.resetValues,
		Description:       opts.description,
		Version:           opts.version,
		RollbackOnFailure: opts.rollbackOnFailure,
	})
	if err != nil {
		return err
	}

	printReleaseResult(result)
	return nil
}
