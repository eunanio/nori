package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/eunanio/nori/pkg/state"
	"github.com/google/go-containerregistry/pkg/v1/remote"
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

// ReleaseInfo represents release information for display.
type ReleaseInfo struct {
	Name        string            `json:"name"`
	Version     string            `json:"version"`
	ModuleRef   string            `json:"module_ref"`
	Status      string            `json:"status"`
	UpdatedAt   time.Time         `json:"updated_at"`
	Annotations map[string]string `json:"annotations,omitempty"`
}

func runReleaseList(cmd *cobra.Command, opts *releaseListOptions) error {
	ctx := cmd.Context()
	log := getLogger()
	cfg := getConfig()

	// Get state repository
	stateRepo, err := cfg.GetStateRepository()
	if err != nil {
		return fmt.Errorf("state repository not configured: %w\nRun: nori config set state_repository <oci-repo>", err)
	}

	log.Debug("listing releases from OCI", "repository", stateRepo)

	// List all repositories under the state repository
	releases, err := listReleasesFromOCI(ctx, stateRepo)
	if err != nil {
		return fmt.Errorf("failed to list releases: %w", err)
	}

	// Filter if not showing all
	if !opts.all {
		var filtered []ReleaseInfo
		for _, r := range releases {
			if r.Status != string(state.StatusFailed) {
				filtered = append(filtered, r)
			}
		}
		releases = filtered
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

// listReleasesFromOCI lists all releases from the OCI state repository.
func listReleasesFromOCI(ctx context.Context, stateRepo string) ([]ReleaseInfo, error) {
	// Parse the repository
	repo, err := getClient().NewRepository(stateRepo)
	if err != nil {
		return nil, fmt.Errorf("invalid repository: %w", err)
	}

	// Get remote options with authentication
	opts, err := getClient().RemoteOptions(ctx, repo.RegistryStr())
	if err != nil {
		return nil, fmt.Errorf("failed to get remote options: %w", err)
	}

	// List tags in the repository
	// Note: This lists tags from the base repo, which contain release-version tags
	tags, err := remote.List(repo, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to list tags: %w", err)
	}

	// Group tags by release name
	releaseVersions := make(map[string][]string)
	for _, tag := range tags {
		// Tags are in format: <release-name>-<version>
		parts := strings.Split(tag, "-v")
		if len(parts) >= 2 {
			releaseName := parts[0]
			version := "v" + parts[len(parts)-1]
			releaseVersions[releaseName] = append(releaseVersions[releaseName], version)
		}
	}

	var releases []ReleaseInfo
	stateStore := state.NewStateStore(getClient(), getLogger())

	for releaseName, versions := range releaseVersions {
		// Get the latest version
		latestVersion, err := state.FindLatestVersion(versions)
		if err != nil {
			continue
		}

		// Build reference for the latest version
		ref := fmt.Sprintf("%s:%s", stateRepo, state.FormatReleaseTag(releaseName, latestVersion))

		// Try to get metadata from the manifest annotations
		parsedRef, err := getClient().ParseReference(ref)
		if err != nil {
			continue
		}

		desc, err := remote.Get(parsedRef, opts...)
		if err != nil {
			continue
		}

		img, err := desc.Image()
		if err != nil {
			continue
		}

		manifest, err := img.Manifest()
		if err != nil {
			continue
		}

		info := ReleaseInfo{
			Name:    releaseName,
			Version: latestVersion,
		}

		if manifest.Annotations != nil {
			if moduleRef, ok := manifest.Annotations["io.nori.module.ref"]; ok {
				info.ModuleRef = moduleRef
			}
			if status, ok := manifest.Annotations["io.nori.release.status"]; ok {
				info.Status = status
			}
			if created, ok := manifest.Annotations["org.opencontainers.image.created"]; ok {
				info.UpdatedAt, _ = time.Parse(time.RFC3339, created)
			}
		}

		// Get user annotations
		info.Annotations = make(map[string]string)
		for k, v := range manifest.Annotations {
			if !isSystemAnnotation(k) {
				info.Annotations[k] = v
			}
		}

		// If we still need data, pull the full state
		if info.ModuleRef == "" || info.Status == "" {
			fullState, err := stateStore.PullState(ctx, ref)
			if err == nil && fullState.Metadata != nil {
				info.ModuleRef = fullState.Metadata.ModuleRef
				info.Status = string(fullState.Metadata.Status)
				info.UpdatedAt = fullState.Metadata.UpdatedAt
			}
		}

		releases = append(releases, info)
	}

	return releases, nil
}

func outputReleasesInfoTable(releases []ReleaseInfo) error {
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

func outputReleasesInfoJSON(releases []ReleaseInfo) error {
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

// isSystemAnnotation checks if an annotation key is a system annotation.
func isSystemAnnotation(key string) bool {
	systemPrefixes := []string{
		"org.opencontainers.",
		"io.nori.release.",
		"io.nori.module.",
		"io.nori.version",
	}

	for _, prefix := range systemPrefixes {
		if strings.HasPrefix(key, prefix) {
			return true
		}
	}

	return false
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
	ctx := cmd.Context()
	log := getLogger()
	cfg := getConfig()

	stateRepo, err := cfg.GetStateRepository()
	if err != nil {
		return fmt.Errorf("state repository not configured: %w", err)
	}

	stateStore := state.NewStateStore(getClient(), log)
	history, err := stateStore.GetReleaseHistory(ctx, stateRepo, releaseName)
	if err != nil {
		return fmt.Errorf("failed to get release history: %w", err)
	}

	if len(history.Versions) == 0 {
		fmt.Printf("No history found for release %q\n", releaseName)
		return nil
	}

	versions := history.Versions
	if opts.limit > 0 && len(versions) > opts.limit {
		versions = versions[:opts.limit]
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

		for _, v := range versions {
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
	ctx := cmd.Context()
	log := getLogger()
	cfg := getConfig()

	stateRepo, err := cfg.GetStateRepository()
	if err != nil {
		return fmt.Errorf("state repository not configured: %w", err)
	}

	stateStore := state.NewStateStore(getClient(), log)

	// Get version to inspect
	version := opts.version
	if version == "" {
		version, err = stateStore.GetLatestVersion(ctx, stateRepo, releaseName)
		if err != nil {
			return fmt.Errorf("failed to get latest version: %w", err)
		}
	}

	// Build reference (flat format: repo:releaseName-version)
	ref := fmt.Sprintf("%s:%s", stateRepo, state.FormatReleaseTag(releaseName, version))

	// Pull state
	releaseState, err := stateStore.PullState(ctx, ref)
	if err != nil {
		return fmt.Errorf("failed to pull release state: %w", err)
	}

	switch opts.output {
	case "json":
		// Create a structured output
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
		// YAML-like output
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
			// Indent the values
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
