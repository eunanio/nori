package commands

import (
	"fmt"

	nori "github.com/eunanio/nori/lib"
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
	reference := args[0]
	filePath := args[1]

	annotations, err := nori.ParseAnnotations(opts.annotations)
	if err != nil {
		return err
	}

	result, err := getLibClient().Push(cmd.Context(), reference, filePath, nori.PushOptions{
		Annotations: annotations,
	})
	if err != nil {
		return err
	}

	fmt.Printf("✓ Artifact pushed successfully\n")
	fmt.Printf("  Reference: %s\n", result.Reference)
	fmt.Printf("  Digest:    %s\n", result.Digest)

	return nil
}
