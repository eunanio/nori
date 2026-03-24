package commands

import (
	"encoding/json"
	"fmt"

	nori "github.com/eunanio/nori/lib"
	"github.com/spf13/cobra"
)

type statusOptions struct {
	output      string
	showOutputs bool
}

func newReleaseStatusCommand() *cobra.Command {
	opts := &statusOptions{}

	cmd := &cobra.Command{
		Use:   "status <name>",
		Short: "Display the status of a release",
		Long: `Display detailed status information about a release.

Shows:
- Release metadata (name, module, revision, status)
- Creation and update timestamps
- Values used for deployment
- Terraform outputs (if available)

Examples:
  # Show release status
  nori release status my-bucket

  # Show status with Terraform outputs
  nori release status my-bucket --show-outputs

  # Show status in JSON format
  nori release status my-bucket -o json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runStatus(cmd, args, opts)
		},
	}

	cmd.Flags().StringVarP(&opts.output, "output", "o", "text", "Output format (text, json)")
	cmd.Flags().BoolVar(&opts.showOutputs, "show-outputs", false, "Fetch and display Terraform outputs")

	return cmd
}

func runStatus(cmd *cobra.Command, args []string, opts *statusOptions) error {
	releaseName := args[0]

	result, err := getLibClient().ReleaseStatus(cmd.Context(), releaseName, nori.StatusOptions{
		ShowOutputs: opts.showOutputs,
	})
	if err != nil {
		return err
	}

	switch opts.output {
	case "json":
		return outputStatusJSON(result)
	default:
		return outputStatusText(result)
	}
}

func outputStatusText(r *nori.ReleaseStatusResult) error {
	fmt.Printf("NAME: %s\n", r.Name)
	fmt.Printf("MODULE: %s\n", r.ModuleRef)
	fmt.Printf("REVISION: %d\n", r.Revision)
	fmt.Printf("STATUS: %s\n", r.Status)
	fmt.Printf("CREATED: %s\n", r.CreatedAt.Format("2006-01-02 15:04:05"))
	fmt.Printf("UPDATED: %s\n", r.UpdatedAt.Format("2006-01-02 15:04:05"))

	if r.ValuesFile != "" {
		fmt.Printf("VALUES FILE: %s\n", r.ValuesFile)
	}

	if r.BackendType != "" && r.BackendType != "local" {
		fmt.Printf("BACKEND: %s\n", r.BackendType)
	}

	if len(r.Values) > 0 {
		fmt.Printf("\nVALUES:\n")
		for k, v := range r.Values {
			fmt.Printf("  %s: %v\n", k, v)
		}
	}

	if len(r.Outputs) > 0 {
		fmt.Printf("\nOUTPUTS:\n")
		for k, v := range r.Outputs {
			if m, ok := v.(map[string]interface{}); ok {
				if val, exists := m["value"]; exists {
					fmt.Printf("  %s: %v\n", k, val)
					continue
				}
			}
			fmt.Printf("  %s: %v\n", k, v)
		}
	}

	return nil
}

func outputStatusJSON(r *nori.ReleaseStatusResult) error {
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal status: %w", err)
	}
	fmt.Println(string(data))
	return nil
}
