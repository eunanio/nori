// Package commands provides CLI command implementations.
package commands

import (
	"log/slog"
	"os"
	"strings"

	"github.com/eunanio/nori/pkg/auth"
	"github.com/eunanio/nori/pkg/config"
	"github.com/eunanio/nori/pkg/oci"
	"github.com/spf13/cobra"
)

var (
	// Global flags
	verbose        bool
	configPath     string
	registryConfig string
	insecure       bool

	// Runtime state
	cfg       *config.Config
	logger    *slog.Logger
	ociClient *oci.Client
	credStore *auth.CredentialStore
)

// NewRootCommand creates the root command.
func NewRootCommand(version, commit, date string) *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "nori",
		Short: "OCI-based Terraform/OpenTofu module management",
		Long: `Nori is a tool for packaging and deploying Terraform/OpenTofu modules as OCI artifacts.

It provides a Helm-like workflow for managing infrastructure modules:
  - Package modules as OCI artifacts
  - Push and pull from any OCI-compliant registry
  - Deploy modules with values files
  - Full OpenTofu 1.10+ compatibility`,
		Version: formatVersion(version, commit, date),
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			return setup(cmd, args)
		},
		SilenceUsage: true,
	}

	// Global flags
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose/debug logging")
	rootCmd.PersistentFlags().StringVar(&configPath, "config", "", "Path to Nori configuration file")
	rootCmd.PersistentFlags().StringVar(&registryConfig, "registry-config", "", "Override Docker config location")
	rootCmd.PersistentFlags().BoolVar(&insecure, "insecure", false, "Allow insecure registry connections")

	// Add subcommands
	rootCmd.AddCommand(
		newPackageCommand(),
		newPullCommand(),
		newPushCommand(),
		newListCommand(),
		newInspectCommand(),
		newLoginCommand(),
		newLogoutCommand(),
		newVersionCommand(version, commit, date),
		newReleaseCommand(),
		newConfigCommand(),
	)

	return rootCmd
}

// setup initializes the runtime state.
func setup(cmd *cobra.Command, args []string) error {
	// Skip setup for version and help commands
	if cmd.Name() == "version" || cmd.Name() == "help" {
		return nil
	}

	// Setup logger
	var level slog.Level
	if verbose {
		level = slog.LevelDebug
	} else {
		level = slog.LevelInfo
	}

	handler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: level,
	})
	logger = slog.New(handler)
	slog.SetDefault(logger)

	// Load configuration
	var err error
	cfg, err = config.Load(configPath)
	if err != nil {
		logger.Warn("failed to load config, using defaults", "error", err)
		cfg = config.DefaultConfig()
	}

	// Update log level from config if not overridden by flag
	if !verbose && cfg.Logging.Level == "debug" {
		handler = slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
			Level: slog.LevelDebug,
		})
		logger = slog.New(handler)
		slog.SetDefault(logger)
	}

	// Setup credential store
	dockerConfigPath := registryConfig
	if dockerConfigPath == "" {
		dockerConfigPath = auth.DefaultDockerConfigPath()
	}
	credStore = auth.NewCredentialStore(dockerConfigPath, logger)

	// Setup OCI client
	ociClient = oci.NewClient(
		oci.WithCredentialStore(credStore),
		oci.WithLogger(logger),
		oci.WithInsecure(insecure),
	)

	return nil
}

// formatVersion formats the version string.
func formatVersion(version, commit, date string) string {
	var sb strings.Builder
	sb.WriteString(version)
	if commit != "unknown" {
		sb.WriteString(" (")
		sb.WriteString(commit)
		if date != "unknown" {
			sb.WriteString(", ")
			sb.WriteString(date)
		}
		sb.WriteString(")")
	}
	return sb.String()
}

// getLogger returns the configured logger.
func getLogger() *slog.Logger {
	if logger == nil {
		return slog.Default()
	}
	return logger
}

// getConfig returns the loaded configuration.
func getConfig() *config.Config {
	if cfg == nil {
		return config.DefaultConfig()
	}
	return cfg
}

// getClient returns the OCI client.
func getClient() *oci.Client {
	return ociClient
}

// getCredStore returns the credential store.
func getCredStore() *auth.CredentialStore {
	return credStore
}
