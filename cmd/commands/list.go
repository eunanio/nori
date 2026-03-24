package commands

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newListCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list <repository>",
		Aliases: []string{"ls"},
		Short:   "List available module versions in a registry",
		Long: `List all available tags for a module repository.

Examples:
  # List all versions
  nori list ghcr.io/myorg/s3-bucket

  # List from a private registry
  nori list registry.example.com/modules/vpc`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runList(cmd, args)
		},
	}

	return cmd
}

func runList(cmd *cobra.Command, args []string) error {
	repository := args[0]

	tags, err := getLibClient().ListTags(cmd.Context(), repository)
	if err != nil {
		return err
	}

	if len(tags) == 0 {
		fmt.Printf("No tags found for %s\n", repository)
		return nil
	}

	fmt.Printf("Tags for %s:\n", repository)
	for _, tag := range tags {
		fmt.Printf("  %s\n", tag)
	}

	return nil
}
