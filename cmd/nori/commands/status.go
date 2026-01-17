package commands

import (
	"encoding/json"
	"fmt"

	"github.com/eunanio/nori/pkg/deploy"
	"github.com/eunanio/nori/pkg/release"
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
	ctx := cmd.Context()
	releaseName := args[0]

	log := getLogger()

	store := release.NewStore("")

	rel, err := store.Get(releaseName)
	if err != nil {
		return fmt.Errorf("release %q not found: %w", releaseName, err)
	}

	// Optionally fetch OpenTofu outputs
	var outputs map[string]interface{}
	if opts.showOutputs {
		deployer := deploy.NewDeployer(getClient(), "tofu", log)
		outputs, err = deployer.GetReleaseOutputs(ctx, store, releaseName)
		if err != nil {
			log.Warn("failed to get outputs", "error", err)
		}
	}

	switch opts.output {
	case "json":
		return outputStatusJSON(rel, outputs)
	default:
		return outputStatusText(rel, outputs)
	}
}

func outputStatusText(rel *release.Release, outputs map[string]interface{}) error {
	fmt.Printf("NAME: %s\n", rel.Name)
	fmt.Printf("MODULE: %s\n", rel.ModuleRef)
	fmt.Printf("REVISION: %d\n", rel.Version)
	fmt.Printf("STATUS: %s\n", rel.Status)
	fmt.Printf("CREATED: %s\n", rel.CreatedAt.Format("2006-01-02 15:04:05"))
	fmt.Printf("UPDATED: %s\n", rel.UpdatedAt.Format("2006-01-02 15:04:05"))

	if rel.ValuesFile != "" {
		fmt.Printf("VALUES FILE: %s\n", rel.ValuesFile)
	}

	if rel.BackendType != "" && rel.BackendType != "local" {
		fmt.Printf("BACKEND: %s\n", rel.BackendType)
	}

	if len(rel.Values) > 0 {
		fmt.Printf("\nVALUES:\n")
		for k, v := range rel.Values {
			fmt.Printf("  %s: %v\n", k, v)
		}
	}

	if len(outputs) > 0 {
		fmt.Printf("\nOUTPUTS:\n")
		for k, v := range outputs {
			// Handle Terraform output format
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

type statusOutput struct {
	Name          string                 `json:"name"`
	ModuleRef     string                 `json:"module_ref"`
	Version       int                    `json:"revision"`
	Status        release.Status         `json:"status"`
	CreatedAt     string                 `json:"created_at"`
	UpdatedAt     string                 `json:"updated_at"`
	ValuesFile    string                 `json:"values_file,omitempty"`
	BackendType   string                 `json:"backend_type,omitempty"`
	BackendConfig map[string]string      `json:"backend_config,omitempty"`
	Values        map[string]interface{} `json:"values,omitempty"`
	Outputs       map[string]interface{} `json:"outputs,omitempty"`
}

func outputStatusJSON(rel *release.Release, outputs map[string]interface{}) error {
	out := statusOutput{
		Name:          rel.Name,
		ModuleRef:     rel.ModuleRef,
		Version:       rel.Version,
		Status:        rel.Status,
		CreatedAt:     rel.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:     rel.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
		ValuesFile:    rel.ValuesFile,
		BackendType:   rel.BackendType,
		BackendConfig: rel.BackendConfig,
		Values:        rel.Values,
		Outputs:       outputs,
	}

	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal status: %w", err)
	}
	fmt.Println(string(data))
	return nil
}
