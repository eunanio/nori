package commands

import (
	"encoding/json"
	"fmt"
	"os"

	nori "github.com/eunanio/nori/lib"
	"github.com/eunanio/nori/pkg/oci"
	"github.com/spf13/cobra"
)

type inspectOptions struct {
	format  string
	readme  bool
	verify  bool
	keyPath string
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
	client := getLibClient()

	// Handle --verify flag
	if opts.verify {
		result, err := client.Verify(ctx, reference, nori.VerifyOptions{KeyPath: opts.keyPath})
		if err != nil || !result.HasSignature || (opts.keyPath != "" && !result.Verified) {
			fmt.Println("false")
			os.Exit(1)
			return nil
		}
		fmt.Println("true")
		return nil
	}

	// Handle --readme flag
	if opts.readme {
		inspectResult, err := client.Inspect(ctx, reference, nori.InspectOptions{IncludeReadme: true})
		if err != nil {
			return err
		}
		if len(inspectResult.Readme) == 0 {
			return fmt.Errorf("this package does not contain a README")
		}
		switch opts.format {
		case "json":
			output := struct {
				Reference string `json:"reference"`
				Readme    string `json:"readme"`
			}{Reference: reference, Readme: string(inspectResult.Readme)}
			data, _ := json.MarshalIndent(output, "", "  ")
			fmt.Println(string(data))
		default:
			fmt.Print(string(inspectResult.Readme))
		}
		return nil
	}

	// Default: full inspect
	inspectResult, err := client.Inspect(ctx, reference, nori.InspectOptions{})
	if err != nil {
		return err
	}

	switch opts.format {
	case "json":
		return printInspectJSONFromLib(inspectResult)
	default:
		return printInspectTextFromLib(reference, inspectResult)
	}
}

func printInspectTextFromLib(reference string, r *nori.InspectResult) error {
	fmt.Printf("Artifact: %s\n", reference)
	fmt.Printf("Digest:   %s\n", r.Digest)

	if r.Signature != nil {
		if r.Signature.HasSignature {
			if r.Signature.Verified {
				fmt.Printf("Signature: Verified ✓\n")
				if r.Signature.SignerID != "" {
					fmt.Printf("  Signer: %s\n", r.Signature.SignerID)
				}
			} else if r.Signature.Error != "" {
				fmt.Printf("Signature: Present (unverified: %s)\n", r.Signature.Error)
			} else {
				fmt.Printf("Signature: Present (use --key to verify)\n")
			}
		} else {
			fmt.Printf("Signature: Not signed\n")
		}
	}

	fmt.Printf("\nLayers:\n")
	for i, layer := range r.Layers {
		fmt.Printf("  [%d] %s\n", i, layer.Digest)
		fmt.Printf("      Media Type: %s\n", layer.MediaType)
		fmt.Printf("      Size:       %d bytes\n", layer.Size)
	}
	fmt.Printf("\n")

	if len(r.Annotations) > 0 {
		fmt.Printf("Annotations:\n")
		for k, v := range r.Annotations {
			if len(v) > 100 {
				v = v[:97] + "..."
			}
			fmt.Printf("  %s: %s\n", k, v)
		}
		fmt.Printf("\n")
	}

	if len(r.Config) > 0 {
		var config oci.ModuleConfig
		if err := json.Unmarshal(r.Config, &config); err == nil {
			fmt.Printf("Config:\n")
			fmt.Printf("  Created:      %s\n", config.Created.Format("2006-01-02 15:04:05"))
			fmt.Printf("  Nori Version: %s\n", config.NoriVersion)
			fmt.Printf("  Module Type:  %s\n", config.ModuleType)
		}
	}

	if r.Annotations[oci.AnnotationReadme] == "true" {
		fmt.Printf("\nREADME: Available (use --readme to view)\n")
	}

	return nil
}

func printInspectJSONFromLib(r *nori.InspectResult) error {
	output := struct {
		Reference   string            `json:"reference"`
		Digest      string            `json:"digest"`
		Signature   *nori.SignatureInfo `json:"signature,omitempty"`
		Annotations map[string]string `json:"annotations,omitempty"`
		Layers      []oci.LayerInfo   `json:"layers"`
		Config      json.RawMessage   `json:"config,omitempty"`
	}{
		Reference:   r.Reference,
		Digest:      r.Digest,
		Signature:   r.Signature,
		Annotations: r.Annotations,
		Layers:      r.Layers,
	}

	if len(r.Config) > 0 {
		output.Config = r.Config
	}

	data, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal output: %w", err)
	}
	fmt.Println(string(data))
	return nil
}
