package commands

import (
	"fmt"
	"sort"

	"github.com/google/go-containerregistry/pkg/name"
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
	ctx := cmd.Context()
	repository := args[0]

	log := getLogger()
	log.Info("listing tags", "repository", repository)

	// Parse repository
	repo, err := name.NewRepository(repository)
	if err != nil {
		return fmt.Errorf("invalid repository: %w", err)
	}

	// List tags
	client := getClient()
	tags, err := client.ListTags(ctx, repo)
	if err != nil {
		return fmt.Errorf("failed to list tags: %w", err)
	}

	if len(tags) == 0 {
		fmt.Printf("No tags found for %s\n", repository)
		return nil
	}

	// Sort tags (try semantic versioning sort)
	sort.Slice(tags, func(i, j int) bool {
		return compareVersions(tags[i], tags[j])
	})

	fmt.Printf("Tags for %s:\n", repository)
	for _, tag := range tags {
		fmt.Printf("  %s\n", tag)
	}

	return nil
}

// compareVersions compares two version strings for sorting.
// Returns true if a should come before b.
func compareVersions(a, b string) bool {
	// Simple lexicographic comparison
	// In production, you'd want proper semver parsing
	return a < b
}

