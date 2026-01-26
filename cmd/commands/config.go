package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/eunanio/nori/pkg/config"
	"github.com/eunanio/nori/pkg/signing"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

// Supported config properties and their descriptions
var configProperties = map[string]string{
	"state_repository":      "OCI repository for storing release state (e.g., ghcr.io/org/nori-state)",
	"default_registry":      "Default OCI registry when not specified",
	"deploy.auto_approve":   "Automatically approve apply operations (true/false)",
	"deploy.parallelism":    "Number of parallel operations (integer)",
	"logging.level":         "Log level (debug, info, warn, error)",
	"logging.format":        "Log format (text, json)",
	"signing.key_path":      "Path to private signing key",
	"signing.password_env":  "Environment variable containing signing key password",
}

func newConfigCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage Nori configuration",
		Long: `Manage Nori configuration settings.

Configuration is stored in ~/.nori/config.yaml

Available properties:
  state_repository      OCI repository for storing release state
  default_registry      Default OCI registry when not specified
  deploy.auto_approve   Auto-approve apply operations (true/false)
  deploy.parallelism    Number of parallel operations
  logging.level         Log level (debug, info, warn, error)
  logging.format        Log format (text, json)
  signing.key_path      Path to private signing key
  signing.password_env  Environment variable containing signing key password

Examples:
  # Set state repository
  nori config set state_repository ghcr.io/myorg/nori-state

  # Get current state repository
  nori config get state_repository

  # View all configuration
  nori config get`,
	}

	cmd.AddCommand(newConfigSetCommand())
	cmd.AddCommand(newConfigGetCommand())
	cmd.AddCommand(newGenerateKeyPairCommand())

	return cmd
}

func newConfigSetCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "set <property> <value>",
		Short: "Set a configuration property",
		Long: `Set a configuration property value.

Examples:
  nori config set state_repository ghcr.io/myorg/nori-state
  nori config set deploy.parallelism 20
  nori config set logging.level debug`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runConfigSet(args[0], args[1])
		},
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			if len(args) == 0 {
				var props []string
				for prop := range configProperties {
					props = append(props, prop)
				}
				return props, cobra.ShellCompDirectiveNoFileComp
			}
			return nil, cobra.ShellCompDirectiveNoFileComp
		},
	}

	return cmd
}

func newConfigGetCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get [property]",
		Short: "Get configuration property value(s)",
		Long: `Get a configuration property value, or all values if no property specified.

Examples:
  nori config get                    # Show all configuration
  nori config get state_repository   # Show specific property`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return runConfigGetAll()
			}
			return runConfigGet(args[0])
		},
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			if len(args) == 0 {
				var props []string
				for prop := range configProperties {
					props = append(props, prop)
				}
				return props, cobra.ShellCompDirectiveNoFileComp
			}
			return nil, cobra.ShellCompDirectiveNoFileComp
		},
	}

	return cmd
}

func runConfigSet(property, value string) error {
	// Load existing config
	cfg, err := config.Load("")
	if err != nil {
		cfg = config.DefaultConfig()
	}

	// Set the property
	switch property {
	case "state_repository":
		cfg.StateRepository = value
	case "default_registry":
		cfg.DefaultRegistry = value
	case "deploy.auto_approve":
		boolVal, err := strconv.ParseBool(value)
		if err != nil {
			return fmt.Errorf("invalid boolean value: %s (must be 'true' or 'false')", value)
		}
		cfg.Deploy.AutoApprove = boolVal
	case "deploy.parallelism":
		intVal, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("invalid integer value: %s", value)
		}
		if intVal < 1 {
			return fmt.Errorf("parallelism must be at least 1")
		}
		cfg.Deploy.Parallelism = intVal
	case "logging.level":
		validLevels := []string{"debug", "info", "warn", "error"}
		valid := false
		for _, l := range validLevels {
			if strings.ToLower(value) == l {
				valid = true
				break
			}
		}
		if !valid {
			return fmt.Errorf("invalid log level: %s (must be debug, info, warn, or error)", value)
		}
		cfg.Logging.Level = strings.ToLower(value)
	case "logging.format":
		if value != "text" && value != "json" {
			return fmt.Errorf("invalid log format: %s (must be 'text' or 'json')", value)
		}
		cfg.Logging.Format = value
	case "signing.key_path":
		cfg.Signing.KeyPath = value
	case "signing.password_env":
		cfg.Signing.PasswordEnv = value
	default:
		return fmt.Errorf("unknown property: %s\n\nAvailable properties:\n%s", property, listProperties())
	}

	// Save config
	if err := config.Save(cfg, ""); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Printf("Set %s = %s\n", property, value)
	return nil
}

func runConfigGet(property string) error {
	cfg := getConfig()

	var value string
	switch property {
	case "state_repository":
		value = cfg.StateRepository
	case "default_registry":
		value = cfg.DefaultRegistry
	case "deploy.auto_approve":
		value = strconv.FormatBool(cfg.Deploy.AutoApprove)
	case "deploy.parallelism":
		value = strconv.Itoa(cfg.Deploy.Parallelism)
	case "logging.level":
		value = cfg.Logging.Level
	case "logging.format":
		value = cfg.Logging.Format
	case "signing.key_path":
		value = cfg.Signing.KeyPath
	case "signing.password_env":
		value = cfg.Signing.PasswordEnv
	default:
		return fmt.Errorf("unknown property: %s\n\nAvailable properties:\n%s", property, listProperties())
	}

	if value == "" {
		value = "(not set)"
	}
	fmt.Printf("%s = %s\n", property, value)
	return nil
}

func runConfigGetAll() error {
	cfg := getConfig()

	fmt.Println("Current configuration:")
	fmt.Println()

	// Core settings
	printConfigValue("state_repository", cfg.StateRepository)
	printConfigValue("default_registry", cfg.DefaultRegistry)

	// Deploy settings
	fmt.Println()
	fmt.Println("Deploy:")
	printConfigValue("  auto_approve", strconv.FormatBool(cfg.Deploy.AutoApprove))
	printConfigValue("  parallelism", strconv.Itoa(cfg.Deploy.Parallelism))

	// Logging settings
	fmt.Println()
	fmt.Println("Logging:")
	printConfigValue("  level", cfg.Logging.Level)
	printConfigValue("  format", cfg.Logging.Format)

	// Signing settings
	fmt.Println()
	fmt.Println("Signing:")
	printConfigValue("  key_path", cfg.Signing.KeyPath)
	printConfigValue("  password_env", cfg.Signing.PasswordEnv)

	fmt.Println()
	fmt.Printf("Config file: %s\n", config.DefaultConfigPath())

	return nil
}

func printConfigValue(name, value string) {
	if value == "" || value == "0" {
		value = "(not set)"
	}
	fmt.Printf("  %s: %s\n", name, value)
}

func listProperties() string {
	var sb strings.Builder
	for prop, desc := range configProperties {
		sb.WriteString(fmt.Sprintf("  %s - %s\n", prop, desc))
	}
	return sb.String()
}

func newGenerateKeyPairCommand() *cobra.Command {
	var outputDir string

	cmd := &cobra.Command{
		Use:   "generate-key-pair",
		Short: "Generate a cosign-compatible key pair for signing artifacts",
		Long: `Generate a new cosign-compatible key pair for signing OCI artifacts.

The keys are stored in the specified directory (default: ~/.nori):
  - nori.key     (private key, password-protected)
  - nori.pub     (public key)

After generation, the key path is automatically configured in ~/.nori/config.yaml,
allowing you to use 'nori package --sign' without specifying --key.

Examples:
  # Generate keys in default location (~/.nori)
  nori config generate-key-pair

  # Generate keys in specific directory
  nori config generate-key-pair --output ~/.nori/keys`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runGenerateKeyPair(outputDir)
		},
	}

	// Default to ~/.nori directory
	home, err := os.UserHomeDir()
	defaultDir := "."
	if err == nil {
		defaultDir = filepath.Join(home, ".nori")
	}

	cmd.Flags().StringVarP(&outputDir, "output", "o", defaultDir, "Output directory for key files")

	return cmd
}

func runGenerateKeyPair(outputDir string) error {
	// Prompt for password
	fmt.Print("Enter password for private key: ")
	password1, err := term.ReadPassword(int(os.Stdin.Fd()))
	if err != nil {
		return fmt.Errorf("failed to read password: %w", err)
	}
	fmt.Println()

	fmt.Print("Enter password again: ")
	password2, err := term.ReadPassword(int(os.Stdin.Fd()))
	if err != nil {
		return fmt.Errorf("failed to read password: %w", err)
	}
	fmt.Println()

	// Verify passwords match
	if string(password1) != string(password2) {
		return fmt.Errorf("passwords do not match")
	}

	if len(password1) == 0 {
		return fmt.Errorf("password cannot be empty")
	}

	// Generate the key pair
	result, err := signing.GenerateKeyPair(outputDir, password1)
	if err != nil {
		return fmt.Errorf("failed to generate key pair: %w", err)
	}

	fmt.Println()
	fmt.Println("✓ Key pair generated successfully")
	fmt.Printf("  Private key: %s\n", result.PrivateKeyPath)
	fmt.Printf("  Public key:  %s\n", result.PublicKeyPath)

	// Auto-configure the key path in config
	cfg, err := config.Load("")
	if err != nil {
		cfg = config.DefaultConfig()
	}
	cfg.Signing.KeyPath = result.PrivateKeyPath
	if err := config.Save(cfg, ""); err != nil {
		fmt.Printf("\nWarning: Could not update config: %v\n", err)
	} else {
		fmt.Printf("✓ Config updated: signing.key_path = %s\n", result.PrivateKeyPath)
	}

	fmt.Println()
	fmt.Println("To sign artifacts:")
	fmt.Println("  nori package --sign <reference> <module>")

	return nil
}
