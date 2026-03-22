package commands

import (
	"fmt"
	"path/filepath"

	"github.com/eunanio/nori/pkg/oci"
	"github.com/spf13/cobra"
)

type pullOptions struct {
	output string
}

func newPullCommand() *cobra.Command {
	opts := &pullOptions{}

	cmd := &cobra.Command{
		Use:   "pull <reference>",
		Short: "Pull a module artifact from a registry",
		Long: `Download a module artifact from a registry.

The artifact will be saved as a tar.gz file in the current directory
or at the path specified by --output.

Examples:
  # Pull a module by tag
  nori pull ghcr.io/myorg/s3-bucket:v1.0.0

  # Pull a module by digest
  nori pull ghcr.io/myorg/s3-bucket@sha256:abc123...

  # Pull to a specific output file
  nori pull ghcr.io/myorg/s3-bucket:v1.0.0 --output ./modules/s3.tar.gz`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPull(cmd, args, opts)
		},
	}

	cmd.Flags().StringVarP(&opts.output, "output", "o", "", "Output file path (default: <name>-<tag>.tar.gz)")

	return cmd
}

func runPull(cmd *cobra.Command, args []string, opts *pullOptions) error {
	ctx := cmd.Context()
	reference := args[0]

	log := getLogger()
	log.Info("pulling module", "reference", reference)

	// Parse reference
	ref, err := getClient().ParseReference(reference)
	if err != nil {
		return fmt.Errorf("invalid reference: %w", err)
	}

	// Determine output path
	outputPath := opts.output
	if outputPath == "" {
		repo := oci.RepositoryFromRef(ref)
		tag := oci.TagFromRef(ref)
		if tag == "" {
			tag = "latest"
		}
		// Use repository name as base filename
		repoName := filepath.Base(repo)
		outputPath = fmt.Sprintf("%s-%s.tar.gz", repoName, tag)
	}

	// Pull and save
	client := getClient()
	if err := client.SaveArtifact(ctx, ref, outputPath); err != nil {
		return fmt.Errorf("failed to pull module: %w", err)
	}

	fmt.Printf("✓ Module pulled successfully\n")
	fmt.Printf("  Reference: %s\n", reference)
	fmt.Printf("  Output:    %s\n", outputPath)

	return nil
}
