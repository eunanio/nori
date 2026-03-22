package commands

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/eunanio/nori/pkg/oci"
	"github.com/eunanio/nori/pkg/packaging"
	"github.com/eunanio/nori/pkg/signing"
	"github.com/spf13/cobra"
	"golang.org/x/term"
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
	ctx := cmd.Context()
	reference := args[0]
	archivePath := args[1]

	log := getLogger()
	log.Info("packaging module", "reference", reference, "archive", archivePath)

	// Validate reference
	if err := oci.ValidateReference(reference, getClient().NameOptions()...); err != nil {
		return fmt.Errorf("invalid reference: %w", err)
	}

	// Validate signing options
	if opts.sign && opts.packageOnly {
		return fmt.Errorf("--sign cannot be used with --package-only")
	}

	// If signing without explicit key, try to use key from config
	if opts.sign && opts.keyPath == "" {
		cfg := getConfig()
		if cfg.Signing.KeyPath != "" {
			opts.keyPath = cfg.Signing.KeyPath
		} else {
			return fmt.Errorf("--sign requires --key <path> or signing.key_path in config\n\nTo configure, run:\n  nori config generate-key-pair")
		}
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

	// Create packager
	packager := packaging.NewPackager(getClient(), log)

	packageOpts := packaging.PackageOptions{
		Description: opts.description,
		ConfigPath:  opts.configFile,
		Annotations: annotations,
		Insecure:    insecure,
	}

	// Handle --package-only flag
	if opts.packageOnly {
		return runPackageOnly(ctx, packager, reference, archivePath, packageOpts, opts.output)
	}

	// Package and push the module
	result, err := packager.Package(ctx, reference, archivePath, packageOpts)
	if err != nil {
		return fmt.Errorf("failed to package module: %w", err)
	}

	// Print result
	fmt.Printf("✅ Module packaged successfully\n")
	fmt.Printf("  Reference: %s\n", result.Reference)
	fmt.Printf("  Digest:    %s\n", result.Digest)
	fmt.Printf("  Size:      %d bytes\n", result.Size)

	// Show README status
	if result.Annotations[oci.AnnotationReadme] == "true" {
		fmt.Printf("  README:    Included\n")
	}

	if len(result.Annotations) > 0 {
		fmt.Printf("  Annotations:\n")
		for k, v := range result.Annotations {
			fmt.Printf("    %s: %s\n", k, v)
		}
	}

	// Handle signing if requested
	if opts.sign {
		fmt.Println()
		if err := signArtifact(ctx, reference, opts); err != nil {
			return fmt.Errorf("failed to sign artifact: %w", err)
		}
	}

	return nil
}

// signArtifact signs an OCI artifact.
func signArtifact(ctx context.Context, reference string, opts *packageOptions) error {
	log := getLogger()

	// Parse reference
	ref, err := getClient().ParseReference(reference)
	if err != nil {
		return fmt.Errorf("invalid reference: %w", err)
	}

	// Get remote options
	client := getClient()
	remoteOpts, err := client.RemoteOptions(ctx, oci.RegistryFromRef(ref))
	if err != nil {
		return fmt.Errorf("failed to get remote options: %w", err)
	}

	// Get password - try environment variable first, then prompt
	cfg := getConfig()
	var password []byte

	if cfg.Signing.PasswordEnv != "" {
		if envPassword := os.Getenv(cfg.Signing.PasswordEnv); envPassword != "" {
			password = []byte(envPassword)
		}
	}

	if password == nil {
		// Prompt for key password
		fmt.Print("Enter password for signing key: ")
		password, err = term.ReadPassword(int(os.Stdin.Fd()))
		if err != nil {
			return fmt.Errorf("failed to read password: %w", err)
		}
		fmt.Println()
	}

	// Create signer options
	signerOpts := []signing.SignerOption{
		signing.WithSignerLogger(log),
		signing.WithSignerInsecure(insecure),
		signing.WithKeyPath(opts.keyPath),
		signing.WithPassword(password),
	}

	signer := signing.NewSigner(signerOpts...)

	fmt.Printf("Signing artifact %s...\n", reference)
	signResult, err := signer.Sign(ctx, ref, remoteOpts...)
	if err != nil {
		return err
	}

	fmt.Printf("✅ Artifact signed successfully\n")
	fmt.Printf("  Signature: %s\n", signResult.SignatureRef)

	return nil
}

func runPackageOnly(ctx context.Context, packager *packaging.Packager, reference, archivePath string, opts packaging.PackageOptions, outputPath string) error {
	// Prepare the package without pushing
	result, err := packager.PreparePackage(ctx, archivePath, opts)
	if err != nil {
		return fmt.Errorf("failed to prepare package: %w", err)
	}

	// Determine output filename
	if outputPath == "" {
		outputPath = deriveOutputFilename(reference)
	}

	// Ensure output directory exists
	outputDir := filepath.Dir(outputPath)
	if outputDir != "" && outputDir != "." {
		if err := os.MkdirAll(outputDir, 0755); err != nil {
			return fmt.Errorf("failed to create output directory: %w", err)
		}
	}

	// Write the package to file
	if err := os.WriteFile(outputPath, result.Content, 0644); err != nil {
		return fmt.Errorf("failed to write package file: %w", err)
	}

	// Print result
	fmt.Printf("✓ Module packaged successfully\n")
	fmt.Printf("  Output:    %s\n", outputPath)
	fmt.Printf("  Size:      %d bytes\n", result.Size)

	// Show README status
	if result.ReadmeContent != nil {
		fmt.Printf("  README:    Detected (%d bytes)\n", len(result.ReadmeContent))
	}

	fmt.Printf("\nTo push this package to a registry, run:\n")
	fmt.Printf("  nori push %s %s\n", reference, outputPath)

	return nil
}

// deriveOutputFilename derives the output filename from the reference.
// For example: ghcr.io/myorg/s3-bucket:v1.0.0 -> s3-bucket-v1.0.0.zip
func deriveOutputFilename(reference string) string {
	// Extract name and tag from reference
	// Reference format: registry/namespace/name:tag or registry/namespace/name@digest
	parts := strings.Split(reference, "/")
	nameWithTag := parts[len(parts)-1]

	// Handle tag
	if idx := strings.Index(nameWithTag, ":"); idx != -1 {
		name := nameWithTag[:idx]
		tag := nameWithTag[idx+1:]
		return fmt.Sprintf("%s-%s.zip", name, tag)
	}

	// Handle digest
	if idx := strings.Index(nameWithTag, "@"); idx != -1 {
		name := nameWithTag[:idx]
		return fmt.Sprintf("%s.zip", name)
	}

	// Default: just the name
	return fmt.Sprintf("%s.zip", nameWithTag)
}

func parseAnnotation(ann string) (string, string, error) {
	for i, c := range ann {
		if c == '=' {
			return ann[:i], ann[i+1:], nil
		}
	}
	return "", "", fmt.Errorf("invalid annotation format %q, expected key=value", ann)
}
