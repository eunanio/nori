package commands

import (
	"fmt"

	"github.com/eunanio/nori/pkg/deploy"
	"github.com/eunanio/nori/pkg/release"
	"github.com/eunanio/nori/pkg/state"
	"github.com/spf13/cobra"
)

type uninstallOptions struct {
	autoApprove bool
	parallelism int
	targets     []string
	keepHistory bool
	dryRun      bool
}

func newReleaseUninstallCommand() *cobra.Command {
	opts := &uninstallOptions{}

	cmd := &cobra.Command{
		Use:     "destroy <name>",
		Aliases: []string{"delete", "del", "uninstall", "rm"},
		Short:   "destroy a release",
		Long: `destroy a release and remove its infrastructure.

This command:
1. Runs terraform destroy to remove all resources
2. Deletes the release from local storage
3. Deletes the release state from OCI registry

Examples:
  # destroy a release (will prompt for confirmation)
  nori release destroy my-bucket

  # destroy with auto-approve
  nori release destroy my-bucket --auto-approve

  # Dry run (show what would be destroyed)
  nori release destroy my-bucket --dry-run

  # Keep the release history (only destroy resources)
  nori release destroy my-bucket --keep-history`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUninstall(cmd, args, opts)
		},
	}

	cmd.Flags().BoolVar(&opts.autoApprove, "auto-approve", false, "Automatically approve destroy")
	cmd.Flags().IntVar(&opts.parallelism, "parallelism", 10, "Number of parallel operations")
	cmd.Flags().StringArrayVar(&opts.targets, "target", nil, "Specific resources to target")
	cmd.Flags().BoolVar(&opts.keepHistory, "keep-history", false, "Keep the release record (only destroy resources)")
	cmd.Flags().BoolVar(&opts.dryRun, "dry-run", false, "Show what would be destroyed without actually destroying")

	return cmd
}

func runUninstall(cmd *cobra.Command, args []string, opts *uninstallOptions) error {
	ctx := cmd.Context()
	releaseName := args[0]

	log := getLogger()
	cfg := getConfig()

	store := release.NewStore("")

	// Get existing release
	rel, err := store.Get(releaseName)
	if err != nil {
		return fmt.Errorf("release %q not found: %w", releaseName, err)
	}

	log.Info("destroying release", "name", releaseName, "module", rel.ModuleRef)

	// Update status
	rel.Status = release.StatusDestroying
	if err := store.Save(rel); err != nil {
		log.Warn("failed to update release status", "error", err)
	}

	// Create deployer
	deployer := deploy.NewDeployer(getClient(), "tofu", log)

	if opts.dryRun {
		fmt.Printf("Dry run: would destroy release %q\n", releaseName)
		fmt.Printf("  Module: %s\n", rel.ModuleRef)
		fmt.Printf("  Revision: %d\n", rel.Version)
		fmt.Printf("  WorkDir: %s\n", store.GetWorkDir(releaseName))
		return nil
	}

	// Destroy infrastructure
	if err := deployer.DestroyRelease(ctx, rel, store, deploy.ReleaseDeployOptions{
		AutoApprove: opts.autoApprove,
		Parallelism: opts.parallelism,
		Targets:     opts.targets,
	}); err != nil {
		// Update status to failed
		rel.Status = release.StatusFailed
		store.Save(rel)
		return fmt.Errorf("destroy failed: %w", err)
	}

	// Delete release record unless keeping history
	if !opts.keepHistory {
		// Delete OCI state if configured
		if stateRepo, err := cfg.GetStateRepository(); err == nil {
			log.Info("deleting release state from OCI", "repository", stateRepo)
			stateStore := state.NewStateStore(getClient(), log)
			if err := stateStore.DeleteRelease(ctx, stateRepo, releaseName); err != nil {
				log.Warn("failed to delete OCI state", "error", err)
				fmt.Printf("WARNING: Failed to delete OCI state: %v\n", err)
			} else {
				log.Info("OCI state deleted successfully")
			}
		}

		// Delete local release record
		if err := store.Delete(releaseName); err != nil {
			return fmt.Errorf("failed to delete release record: %w", err)
		}
		fmt.Printf("release %q destroyed\n", releaseName)
	} else {
		// Mark as destroyed but keep record
		rel.Status = release.StatusFailed // Use failed as "destroyed" state
		if err := store.Save(rel); err != nil {
			log.Warn("failed to update release status", "error", err)
		}
		fmt.Printf("release %q resources destroyed (history kept)\n", releaseName)
	}

	return nil
}
