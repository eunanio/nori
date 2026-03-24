package commands

import (
	"fmt"

	nori "github.com/eunanio/nori/lib"
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
	releaseName := args[0]

	result, err := getLibClient().DestroyRelease(cmd.Context(), releaseName, nori.DestroyReleaseOptions{
		AutoApprove: opts.autoApprove,
		Parallelism: opts.parallelism,
		Targets:     opts.targets,
		KeepHistory: opts.keepHistory,
		DryRun:      opts.dryRun,
	})
	if err != nil {
		return err
	}

	if result.DryRun {
		fmt.Printf("Dry run: would destroy release %q\n", releaseName)
		fmt.Printf("  Module: %s\n", result.ModuleRef)
		fmt.Printf("  Revision: %d\n", result.Revision)
		fmt.Printf("  WorkDir: %s\n", result.WorkDir)
		return nil
	}

	if opts.keepHistory {
		fmt.Printf("release %q resources destroyed (history kept)\n", releaseName)
	} else {
		fmt.Printf("release %q destroyed\n", releaseName)
	}

	return nil
}
