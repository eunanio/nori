package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	nori "github.com/eunanio/nori/lib"
	"github.com/spf13/cobra"
)

// newReleaseCommand creates the parent release command with all subcommands.
func newReleaseCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "release",
		Aliases: []string{"rel"},
		Short:   "Manage releases",
		Long: `Manage releases - create, upgrade, destroy, list, and inspect.

A release represents a deployment of a Terraform/OpenTofu module from an OCI registry.
Release state is stored as an OCI artifact for versioning and history tracking.

Examples:
  # Create a new release
  nori release create my-bucket ghcr.io/myorg/s3-bucket:v1.0.0 -f values.yaml

  # Upgrade an existing release
  nori release upgrade my-bucket -f values.yaml

  # List all releases
  nori release list

  # Destroy a release
  nori release destroy my-bucket`,
	}

	// Add subcommands
	cmd.AddCommand(newReleaseCreateCommand())
	cmd.AddCommand(newReleaseUpgradeCommand())
	cmd.AddCommand(newReleaseUninstallCommand())
	cmd.AddCommand(newReleaseListCommand())
	cmd.AddCommand(newReleaseHistoryCommand())
	cmd.AddCommand(newReleaseInspectCommand())
	cmd.AddCommand(newReleaseStatusCommand())

	return cmd
}

type releaseListOptions struct {
	output string
	all    bool
}

func newReleaseListCommand() *cobra.Command {
	opts := &releaseListOptions{}

	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List all releases from the OCI state repository",
		Long: `List all releases stored in the configured OCI state repository.

Displays information about each release including:
- Name
- Latest version
- Module reference
- Status
- Last updated time

Examples:
  # List all releases
  nori release list

  # List releases in JSON format
  nori release list -o json

  # Show all releases including failed
  nori release list -a`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runReleaseList(cmd, opts)
		},
	}

	cmd.Flags().StringVarP(&opts.output, "output", "o", "table", "Output format (table, json)")
	cmd.Flags().BoolVarP(&opts.all, "all", "a", false, "Show all releases including failed")

	return cmd
}

func runReleaseList(cmd *cobra.Command, opts *releaseListOptions) error {
	releases, err := getLibClient().ListReleases(cmd.Context(), nori.ListReleasesOptions{
		All: opts.all,
	})
	if err != nil {
		return err
	}

	if len(releases) == 0 {
		fmt.Println("No releases found.")
		return nil
	}

	switch opts.output {
	case "json":
		return outputReleasesInfoJSON(releases)
	default:
		return outputReleasesInfoTable(releases)
	}
}

func outputReleasesInfoTable(releases []nori.ReleaseInfo) error {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tVERSION\tMODULE\tSTATUS\tUPDATED")

	for _, r := range releases {
		updated := formatTimeAgo(r.UpdatedAt)
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
			r.Name,
			r.Version,
			truncateString(r.ModuleRef, 45),
			r.Status,
			updated,
		)
	}

	return w.Flush()
}

func outputReleasesInfoJSON(releases []nori.ReleaseInfo) error {
	data, err := json.MarshalIndent(releases, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal releases: %w", err)
	}
	fmt.Println(string(data))
	return nil
}

func formatTimeAgo(t time.Time) string {
	if t.IsZero() {
		return "unknown"
	}

	diff := time.Since(t)

	switch {
	case diff < time.Minute:
		return "just now"
	case diff < time.Hour:
		mins := int(diff.Minutes())
		if mins == 1 {
			return "1 minute ago"
		}
		return fmt.Sprintf("%d minutes ago", mins)
	case diff < 24*time.Hour:
		hours := int(diff.Hours())
		if hours == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", hours)
	case diff < 7*24*time.Hour:
		days := int(diff.Hours() / 24)
		if days == 1 {
			return "1 day ago"
		}
		return fmt.Sprintf("%d days ago", days)
	default:
		return t.Format("2006-01-02")
	}
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// Release History subcommand
type releaseHistoryOptions struct {
	output string
	limit  int
}

func newReleaseHistoryCommand() *cobra.Command {
	opts := &releaseHistoryOptions{}

	cmd := &cobra.Command{
		Use:   "history <release_name>",
		Short: "Show version history for a release",
		Long: `Show the version history of a release from the OCI state repository.

Examples:
  # Show history for a release
  nori releases history my-bucket

  # Show history in JSON format
  nori releases history my-bucket -o json

  # Limit to last 5 versions
  nori releases history my-bucket --limit 5`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runReleaseHistory(cmd, args[0], opts)
		},
	}

	cmd.Flags().StringVarP(&opts.output, "output", "o", "table", "Output format (table, json)")
	cmd.Flags().IntVar(&opts.limit, "limit", 0, "Limit number of versions to show (0 = all)")

	return cmd
}

func runReleaseHistory(cmd *cobra.Command, releaseName string, opts *releaseHistoryOptions) error {
	history, err := getLibClient().ReleaseHistory(cmd.Context(), releaseName, nori.HistoryOptions{
		Limit: opts.limit,
	})
	if err != nil {
		return err
	}

	if len(history.Versions) == 0 {
		fmt.Printf("No history found for release %q\n", releaseName)
		return nil
	}

	switch opts.output {
	case "json":
		data, err := json.MarshalIndent(history, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal history: %w", err)
		}
		fmt.Println(string(data))
	default:
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintf(w, "RELEASE: %s\n\n", releaseName)
		fmt.Fprintln(w, "VERSION\tSTATUS\tMODULE\tCREATED")

		for _, v := range history.Versions {
			created := formatTimeAgo(v.CreatedAt)
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
				v.Version,
				v.Status,
				truncateString(v.ModuleRef, 45),
				created,
			)
		}

		return w.Flush()
	}

	return nil
}

// Release Inspect subcommand
type releaseInspectOptions struct {
	output  string
	version string
}

func newReleaseInspectCommand() *cobra.Command {
	opts := &releaseInspectOptions{}

	cmd := &cobra.Command{
		Use:   "inspect <release_name>",
		Short: "Show detailed information about a release",
		Long: `Show detailed information about a release including metadata, annotations, and values.

Examples:
  # Inspect a release (latest version)
  nori releases inspect my-bucket

  # Inspect a specific version
  nori releases inspect my-bucket --version v1.2.0

  # Output as JSON
  nori releases inspect my-bucket -o json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runReleaseInspect(cmd, args[0], opts)
		},
	}

	cmd.Flags().StringVarP(&opts.output, "output", "o", "yaml", "Output format (yaml, json)")
	cmd.Flags().StringVar(&opts.version, "version", "", "Specific version to inspect (default: latest)")

	return cmd
}

func runReleaseInspect(cmd *cobra.Command, releaseName string, opts *releaseInspectOptions) error {
	releaseState, err := getLibClient().InspectRelease(cmd.Context(), releaseName, nori.InspectReleaseOptions{
		Version: opts.version,
	})
	if err != nil {
		return err
	}

	switch opts.output {
	case "json":
		output := map[string]interface{}{
			"metadata": releaseState.Metadata,
			"values":   string(releaseState.Values),
		}
		data, err := json.MarshalIndent(output, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal: %w", err)
		}
		fmt.Println(string(data))
	default:
		fmt.Printf("Name: %s\n", releaseState.Metadata.Name)
		fmt.Printf("Version: %s\n", releaseState.Metadata.Version)
		fmt.Printf("Module: %s\n", releaseState.Metadata.ModuleRef)
		fmt.Printf("Status: %s\n", releaseState.Metadata.Status)
		fmt.Printf("Created: %s\n", releaseState.Metadata.CreatedAt.Format(time.RFC3339))
		fmt.Printf("Updated: %s\n", releaseState.Metadata.UpdatedAt.Format(time.RFC3339))

		if releaseState.Metadata.Description != "" {
			fmt.Printf("Description: %s\n", releaseState.Metadata.Description)
		}

		if len(releaseState.Metadata.Annotations) > 0 {
			fmt.Printf("\nAnnotations:\n")
			for k, v := range releaseState.Metadata.Annotations {
				fmt.Printf("  %s: %s\n", k, v)
			}
		}

		if len(releaseState.Values) > 0 {
			fmt.Printf("\nValues:\n")
			lines := strings.Split(string(releaseState.Values), "\n")
			for _, line := range lines {
				if line != "" {
					fmt.Printf("  %s\n", line)
				}
			}
		}
	}

	return nil
}
