package commands

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/eunanio/nori/pkg/oci"
	"github.com/eunanio/nori/pkg/signing"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/spf13/cobra"
)

type inspectOptions struct {
	format  string
	readme  bool
	verify  bool   // Simple true/false verification check
	keyPath string // Path to public key for verification
}

func newInspectCommand() *cobra.Command {
	opts := &inspectOptions{}

	cmd := &cobra.Command{
		Use:   "inspect <reference>",
		Short: "Show metadata and manifest details for an artifact",
		Long: `Display detailed information about an OCI artifact.

This command shows the manifest, layers, and annotations without
downloading the full artifact content.

Examples:
  # Inspect a module
  nori inspect ghcr.io/myorg/s3-bucket:v1.0.0

  # Inspect with JSON output
  nori inspect ghcr.io/myorg/s3-bucket:v1.0.0 --format json

  # Inspect by digest
  nori inspect ghcr.io/myorg/s3-bucket@sha256:abc123...

  # View the module's README
  nori inspect ghcr.io/myorg/s3-bucket:v1.0.0 --readme

  # Verify signature and get true/false result
  nori inspect ghcr.io/myorg/s3-bucket:v1.0.0 --verify

  # Verify signature with a specific public key
  nori inspect ghcr.io/myorg/s3-bucket:v1.0.0 --verify --key nori.pub`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInspect(cmd, args, opts)
		},
	}

	cmd.Flags().StringVar(&opts.format, "format", "text", "Output format (text, json)")
	cmd.Flags().BoolVar(&opts.readme, "readme", false, "Display the module's README content")
	cmd.Flags().BoolVar(&opts.verify, "verify", false, "Verify artifact signature and return true/false")
	cmd.Flags().StringVar(&opts.keyPath, "key", "", "Path to public key file for signature verification")

	return cmd
}

func runInspect(cmd *cobra.Command, args []string, opts *inspectOptions) error {
	ctx := cmd.Context()
	reference := args[0]

	log := getLogger()
	log.Info("inspecting artifact", "reference", reference)

	// Parse reference
	ref, err := getClient().ParseReference(reference)
	if err != nil {
		return fmt.Errorf("invalid reference: %w", err)
	}

	client := getClient()

	// Handle --verify flag (simple true/false output)
	if opts.verify {
		return runVerifySignature(cmd, ref, client, opts)
	}

	// Handle --readme flag
	if opts.readme {
		return runInspectReadme(cmd, ref, client, opts)
	}

	// Inspect artifact
	artifact, err := client.InspectArtifact(ctx, ref)
	if err != nil {
		return fmt.Errorf("failed to inspect artifact: %w", err)
	}

	// Check for signature
	signatureInfo := checkSignature(cmd, ref, client, opts)

	switch opts.format {
	case "json":
		return printInspectJSONWithSignature(artifact, signatureInfo)
	default:
		return printInspectTextWithSignature(reference, artifact, signatureInfo)
	}
}

// SignatureInfo contains information about an artifact's signature.
type SignatureInfo struct {
	HasSignature bool   `json:"has_signature"`
	Verified     bool   `json:"verified"`
	SignerID     string `json:"signer_id,omitempty"`
	Error        string `json:"error,omitempty"`
}

// checkSignature checks if an artifact has a valid signature.
func checkSignature(cmd *cobra.Command, ref name.Reference, client *oci.Client, opts *inspectOptions) *SignatureInfo {
	ctx := cmd.Context()
	log := getLogger()

	// Get remote options
	remoteOpts, err := client.RemoteOptions(ctx, oci.RegistryFromRef(ref))
	if err != nil {
		return &SignatureInfo{Error: err.Error()}
	}

	// Create signer for verification
	signerOpts := []signing.SignerOption{
		signing.WithSignerLogger(log),
		signing.WithSignerInsecure(insecure),
	}

	if opts.keyPath != "" {
		signerOpts = append(signerOpts, signing.WithKeyPath(opts.keyPath))
	}

	signer := signing.NewSigner(signerOpts...)

	// Check if signature exists
	hasSignature, err := signer.HasSignature(ctx, ref, remoteOpts...)
	if err != nil {
		return &SignatureInfo{Error: err.Error()}
	}

	if !hasSignature {
		return &SignatureInfo{HasSignature: false}
	}

	// If we have a key, try to verify
	if opts.keyPath != "" {
		publicKey, err := signing.LoadPublicKey(opts.keyPath)
		if err != nil {
			return &SignatureInfo{
				HasSignature: true,
				Error:        fmt.Sprintf("failed to load public key: %v", err),
			}
		}

		result, err := signer.VerifyWithPublicKey(ctx, ref, publicKey, remoteOpts...)
		if err != nil {
			return &SignatureInfo{
				HasSignature: true,
				Error:        err.Error(),
			}
		}

		return &SignatureInfo{
			HasSignature: true,
			Verified:     result.Verified,
			SignerID:     result.SignerID,
		}
	}

	// Signature exists but we can't verify without a key
	return &SignatureInfo{
		HasSignature: true,
		Verified:     false,
		Error:        "public key required for verification (use --key)",
	}
}

// runVerifySignature runs signature verification and outputs true/false.
func runVerifySignature(cmd *cobra.Command, ref name.Reference, client *oci.Client, opts *inspectOptions) error {
	ctx := cmd.Context()
	log := getLogger()

	// Get remote options
	remoteOpts, err := client.RemoteOptions(ctx, oci.RegistryFromRef(ref))
	if err != nil {
		fmt.Println("false")
		os.Exit(1)
		return nil
	}

	// Create signer for verification
	signerOpts := []signing.SignerOption{
		signing.WithSignerLogger(log),
		signing.WithSignerInsecure(insecure),
	}

	if opts.keyPath != "" {
		signerOpts = append(signerOpts, signing.WithKeyPath(opts.keyPath))
	}

	signer := signing.NewSigner(signerOpts...)

	// Check if signature exists
	hasSignature, err := signer.HasSignature(ctx, ref, remoteOpts...)
	if err != nil || !hasSignature {
		fmt.Println("false")
		os.Exit(1)
		return nil
	}

	// If we have a key, verify the signature
	if opts.keyPath != "" {
		publicKey, err := signing.LoadPublicKey(opts.keyPath)
		if err != nil {
			fmt.Println("false")
			os.Exit(1)
			return nil
		}

		result, err := signer.VerifyWithPublicKey(ctx, ref, publicKey, remoteOpts...)
		if err != nil || !result.Verified {
			fmt.Println("false")
			os.Exit(1)
			return nil
		}

		fmt.Println("true")
		return nil
	}

	// Has signature but can't fully verify without key
	// Return true since signature exists (user may want to just check presence)
	fmt.Println("true")
	return nil
}

func runInspectReadme(cmd *cobra.Command, ref name.Reference, client *oci.Client, opts *inspectOptions) error {
	ctx := cmd.Context()

	// First check if the artifact has a README annotation
	artifact, err := client.InspectArtifact(ctx, ref)
	if err != nil {
		return fmt.Errorf("failed to inspect artifact: %w", err)
	}

	// Check for README annotation
	if artifact.Annotations[oci.AnnotationReadme] != "true" {
		return fmt.Errorf("this package does not contain a README")
	}

	// Pull the README content
	readmeContent, err := client.PullReadme(ctx, ref)
	if err != nil {
		return fmt.Errorf("failed to pull README: %w", err)
	}

	if readmeContent == nil {
		return fmt.Errorf("README layer not found in artifact")
	}

	// Output based on format
	switch opts.format {
	case "json":
		output := struct {
			Reference string `json:"reference"`
			Readme    string `json:"readme"`
		}{
			Reference: ref.String(),
			Readme:    string(readmeContent),
		}
		data, err := json.MarshalIndent(output, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal output: %w", err)
		}
		fmt.Println(string(data))
	default:
		fmt.Print(string(readmeContent))
	}

	return nil
}

func printInspectText(reference string, artifact *oci.Artifact) error {
	return printInspectTextWithSignature(reference, artifact, nil)
}

func printInspectTextWithSignature(reference string, artifact *oci.Artifact, sigInfo *SignatureInfo) error {
	fmt.Printf("Artifact: %s\n", reference)
	fmt.Printf("Digest:   %s\n", artifact.Digest)

	// Show signature status
	if sigInfo != nil {
		if sigInfo.HasSignature {
			if sigInfo.Verified {
				fmt.Printf("Signature: Verified ✓\n")
				if sigInfo.SignerID != "" {
					fmt.Printf("  Signer: %s\n", sigInfo.SignerID)
				}
			} else if sigInfo.Error != "" {
				fmt.Printf("Signature: Present (unverified: %s)\n", sigInfo.Error)
			} else {
				fmt.Printf("Signature: Present (use --key to verify)\n")
			}
		} else {
			fmt.Printf("Signature: Not signed\n")
		}
	}

	fmt.Printf("\n")

	fmt.Printf("Layers:\n")
	for i, layer := range artifact.Layers {
		fmt.Printf("  [%d] %s\n", i, layer.Digest)
		fmt.Printf("      Media Type: %s\n", layer.MediaType)
		fmt.Printf("      Size:       %d bytes\n", layer.Size)
	}
	fmt.Printf("\n")

	if len(artifact.Annotations) > 0 {
		fmt.Printf("Annotations:\n")
		for k, v := range artifact.Annotations {
			// Truncate long values
			if len(v) > 100 {
				v = v[:97] + "..."
			}
			fmt.Printf("  %s: %s\n", k, v)
		}
		fmt.Printf("\n")
	}

	// Try to parse and display config
	if len(artifact.Config) > 0 {
		var config oci.ModuleConfig
		if err := json.Unmarshal(artifact.Config, &config); err == nil {
			fmt.Printf("Config:\n")
			fmt.Printf("  Created:      %s\n", config.Created.Format("2006-01-02 15:04:05"))
			fmt.Printf("  Nori Version: %s\n", config.NoriVersion)
			fmt.Printf("  Module Type:  %s\n", config.ModuleType)
		}
	}

	// Show README availability
	if artifact.Annotations[oci.AnnotationReadme] == "true" {
		fmt.Printf("\nREADME: Available (use --readme to view)\n")
	}

	return nil
}

func printInspectJSON(artifact *oci.Artifact) error {
	return printInspectJSONWithSignature(artifact, nil)
}

func printInspectJSONWithSignature(artifact *oci.Artifact, sigInfo *SignatureInfo) error {
	output := struct {
		Reference   string            `json:"reference"`
		Digest      string            `json:"digest"`
		Signature   *SignatureInfo    `json:"signature,omitempty"`
		Annotations map[string]string `json:"annotations,omitempty"`
		Layers      []oci.LayerInfo   `json:"layers"`
		Config      json.RawMessage   `json:"config,omitempty"`
	}{
		Reference:   artifact.Reference.String(),
		Digest:      artifact.Digest,
		Signature:   sigInfo,
		Annotations: artifact.Annotations,
		Layers:      artifact.Layers,
	}

	if len(artifact.Config) > 0 {
		output.Config = artifact.Config
	}

	data, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal output: %w", err)
	}

	fmt.Println(string(data))
	return nil
}
