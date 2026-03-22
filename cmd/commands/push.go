package commands

import (
	"fmt"
	"os"

	"github.com/eunanio/nori/internal/util"
	"github.com/spf13/cobra"
)

type pushOptions struct {
	annotations []string
}

func newPushCommand() *cobra.Command {
	opts := &pushOptions{}

	cmd := &cobra.Command{
		Use:   "push <reference> <file>",
		Short: "Push a local artifact to a registry",
		Long: `Push a local archive file to a registry.

This is similar to 'nori package' but allows pushing pre-built artifacts.
The file must be a .zip or .tar.gz archive.

Examples:
  # Push a tar.gz file
  nori push ghcr.io/myorg/s3-bucket:v1.0.0 module.tar.gz

  # Push with annotations
  nori push ghcr.io/myorg/s3-bucket:v1.0.0 module.tar.gz \
    --annotation org.opencontainers.image.description="My module"`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPush(cmd, args, opts)
		},
	}

	cmd.Flags().StringArrayVar(&opts.annotations, "annotation", nil, "Custom OCI annotations (key=value)")

	return cmd
}

func runPush(cmd *cobra.Command, args []string, opts *pushOptions) error {
	ctx := cmd.Context()
	reference := args[0]
	filePath := args[1]

	log := getLogger()
	log.Info("pushing artifact", "reference", reference, "file", filePath)

	// Validate file exists
	if _, err := os.Stat(filePath); err != nil {
		return fmt.Errorf("file not found: %w", err)
	}

	// Validate archive format
	if err := util.ValidateArchive(filePath); err != nil {
		return fmt.Errorf("invalid archive: %w", err)
	}

	// Parse reference
	ref, err := getClient().ParseReference(reference)
	if err != nil {
		return fmt.Errorf("invalid reference: %w", err)
	}

	// Parse annotations
	annotations := make(map[string]string)
	for _, ann := range opts.annotations {
		key, value, err := parseAnnotation(ann)
		if err != nil {
			return err
		}
		annotations[key] = value
	}

	// Push artifact
	client := getClient()
	artifact, err := client.LoadAndPushArtifact(ctx, ref, filePath, annotations)
	if err != nil {
		return fmt.Errorf("failed to push artifact: %w", err)
	}

	fmt.Printf("✓ Artifact pushed successfully\n")
	fmt.Printf("  Reference: %s\n", reference)
	fmt.Printf("  Digest:    %s\n", artifact.Digest)

	return nil
}
