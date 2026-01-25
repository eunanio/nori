package commands

import (
	"encoding/json"
	"fmt"

	"github.com/eunanio/nori/pkg/oci"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/spf13/cobra"
)

type inspectOptions struct {
	format string
	readme bool
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
  nori inspect ghcr.io/myorg/s3-bucket:v1.0.0 --readme`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInspect(cmd, args, opts)
		},
	}

	cmd.Flags().StringVar(&opts.format, "format", "text", "Output format (text, json)")
	cmd.Flags().BoolVar(&opts.readme, "readme", false, "Display the module's README content")

	return cmd
}

func runInspect(cmd *cobra.Command, args []string, opts *inspectOptions) error {
	ctx := cmd.Context()
	reference := args[0]

	log := getLogger()
	log.Info("inspecting artifact", "reference", reference)

	// Parse reference
	ref, err := name.ParseReference(reference)
	if err != nil {
		return fmt.Errorf("invalid reference: %w", err)
	}

	client := getClient()

	// Handle --readme flag
	if opts.readme {
		return runInspectReadme(cmd, ref, client, opts)
	}

	// Inspect artifact
	artifact, err := client.InspectArtifact(ctx, ref)
	if err != nil {
		return fmt.Errorf("failed to inspect artifact: %w", err)
	}

	switch opts.format {
	case "json":
		return printInspectJSON(artifact)
	default:
		return printInspectText(reference, artifact)
	}
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
	fmt.Printf("Artifact: %s\n", reference)
	fmt.Printf("Digest:   %s\n", artifact.Digest)
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
	output := struct {
		Reference   string            `json:"reference"`
		Digest      string            `json:"digest"`
		Annotations map[string]string `json:"annotations,omitempty"`
		Layers      []oci.LayerInfo   `json:"layers"`
		Config      json.RawMessage   `json:"config,omitempty"`
	}{
		Reference:   artifact.Reference.String(),
		Digest:      artifact.Digest,
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
