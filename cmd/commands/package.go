package commands

import (
	"fmt"

	nori "github.com/eunanio/nori/lib"
	"github.com/eunanio/nori/pkg/oci"
	"github.com/spf13/cobra"
)

type packageOptions struct {
	configFile  string
	description string
	annotations []string
	packageOnly bool
	output      string
	// Signing options
	sign    bool
	keyPath string
}

func newPackageCommand() *cobra.Command {
	opts := &packageOptions{}

	cmd := &cobra.Command{
		Use:   "package <registry>/<namespace>/<name>:<tag> <module-file>",
		Short: "Package a module archive into an OCI artifact",
		Long: `Package a Terraform/OpenTofu module archive (.zip or .tar.gz) and push it
to an docker registry as an artifact.

The packaged artifact is compatible with:
  - OpenTofu 1.10+ native OCI module support

Examples:
  # Package a zip file
  nori package ghcr.io/myorg/s3-bucket:v1.0.0 module.zip

  # Package with description
  nori package ghcr.io/myorg/s3-bucket:v1.0.0 module.tar.gz \
    --description "Creates an S3 bucket with versioning"

  # Package with custom annotations
  nori package ghcr.io/myorg/s3-bucket:v1.0.0 module.zip \
    --annotation org.opencontainers.image.authors="team@example.com"

  # Extract metadata from Terraform config
  nori package ghcr.io/myorg/s3-bucket:v1.0.0 module.zip \
    --config ./modules/s3-bucket

  # Package without pushing (write to local file)
  nori package ghcr.io/myorg/s3-bucket:v1.0.0 module.tar.gz --package-only

  # Package to a specific output file
  nori package ghcr.io/myorg/s3-bucket:v1.0.0 module.tar.gz --package-only --output ./dist/module.zip

  # Package and sign (uses key from config if configured)
  nori package ghcr.io/myorg/s3-bucket:v1.0.0 module.zip --sign

  # Package and sign with explicit key file
  nori package ghcr.io/myorg/s3-bucket:v1.0.0 module.zip --sign --key nori.key`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPackage(cmd, args, opts)
		},
	}

	cmd.Flags().StringVar(&opts.configFile, "config", "", "Path to Terraform/OpenTofu configuration for metadata extraction")
	cmd.Flags().StringVar(&opts.description, "description", "", "Module description")
	cmd.Flags().StringArrayVar(&opts.annotations, "annotation", nil, "Custom OCI annotations (key=value)")
	cmd.Flags().BoolVar(&opts.packageOnly, "package-only", false, "Package without pushing; write artifact to local file")
	cmd.Flags().StringVarP(&opts.output, "output", "o", "", "Output file path (used with --package-only)")

	// Signing flags
	cmd.Flags().BoolVar(&opts.sign, "sign", false, "Sign the artifact after pushing (uses key from config if not specified)")
	cmd.Flags().StringVar(&opts.keyPath, "key", "", "Path to private key file for signing (overrides config)")

	return cmd
}

func runPackage(cmd *cobra.Command, args []string, opts *packageOptions) error {
	reference := args[0]
	archivePath := args[1]

	annotations, err := nori.ParseAnnotations(opts.annotations)
	if err != nil {
		return err
	}

	result, err := getLibClient().Package(cmd.Context(), reference, archivePath, nori.PackageOptions{
		Description: opts.description,
		ConfigPath:  opts.configFile,
		Annotations: annotations,
		PackageOnly: opts.packageOnly,
		OutputPath:  opts.output,
		Sign:        opts.sign,
		SignKeyPath: opts.keyPath,
	})
	if err != nil {
		return err
	}

	if opts.packageOnly {
		fmt.Printf("✓ Module packaged successfully\n")
		fmt.Printf("  Output:    %s\n", result.LocalPath)
		fmt.Printf("  Size:      %d bytes\n", result.Size)
		fmt.Printf("\nTo push this package to a registry, run:\n")
		fmt.Printf("  nori push %s %s\n", reference, result.LocalPath)
		return nil
	}

	fmt.Printf("✅ Module packaged successfully\n")
	fmt.Printf("  Reference: %s\n", result.Reference)
	fmt.Printf("  Digest:    %s\n", result.Digest)
	fmt.Printf("  Size:      %d bytes\n", result.Size)

	if result.Annotations[oci.AnnotationReadme] == "true" {
		fmt.Printf("  README:    Included\n")
	}

	if len(result.Annotations) > 0 {
		fmt.Printf("  Annotations:\n")
		for k, v := range result.Annotations {
			fmt.Printf("    %s: %s\n", k, v)
		}
	}

	if result.Signed {
		fmt.Printf("\n✅ Artifact signed successfully\n")
		fmt.Printf("  Signature: %s\n", result.SignatureRef)
	}

	return nil
}
