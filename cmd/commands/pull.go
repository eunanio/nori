package commands

import (
	"fmt"

	nori "github.com/eunanio/nori/lib"
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
	reference := args[0]

	result, err := getLibClient().Pull(cmd.Context(), reference, nori.PullOptions{
		OutputPath: opts.output,
	})
	if err != nil {
		return err
	}

	fmt.Printf("✓ Module pulled successfully\n")
	fmt.Printf("  Reference: %s\n", result.Reference)
	fmt.Printf("  Output:    %s\n", result.OutputPath)

	return nil
}
