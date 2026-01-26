// Package config provides configuration management for Nori.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config is the main configuration structure for Nori.
type Config struct {
	// DefaultRegistry is the default registry to use when not specified.
	DefaultRegistry string `yaml:"default_registry,omitempty"`

	// StateRepository is the OCI repository path for storing release state.
	// Example: "ghcr.io/org/nori-state"
	// Release states will be stored as: <state_repository>/<release-name>:<semver>
	StateRepository string `yaml:"state_repository,omitempty"`

	// Registries contains registry-specific configurations.
	Registries map[string]RegistryConfig `yaml:"registries,omitempty"`

	// Deploy contains deployment-specific configurations.
	Deploy DeployConfig `yaml:"deploy,omitempty"`

	// Logging contains logging configuration.
	Logging LoggingConfig `yaml:"logging,omitempty"`

	// Signing contains signing configuration.
	Signing SigningConfig `yaml:"signing,omitempty"`
}

// RegistryConfig contains configuration for a specific registry.
type RegistryConfig struct {
	// Insecure allows HTTP connections to this registry.
	Insecure bool `yaml:"insecure,omitempty"`

	// SkipTLSVerify skips TLS certificate verification.
	SkipTLSVerify bool `yaml:"skip_tls_verify,omitempty"`

	// Username for authentication.
	Username string `yaml:"username,omitempty"`

	// Password or token for authentication.
	Password string `yaml:"password,omitempty"`
}

// DeployConfig contains deployment configuration.
type DeployConfig struct {
	// DefaultWorkDir is the default working directory for deployments.
	DefaultWorkDir string `yaml:"default_workdir,omitempty"`

	// AutoApprove automatically approves apply operations.
	AutoApprove bool `yaml:"auto_approve,omitempty"`

	// Parallelism is the number of parallel operations.
	Parallelism int `yaml:"parallelism,omitempty"`
}

// LoggingConfig contains logging configuration.
type LoggingConfig struct {
	// Level is the log level (debug, info, warn, error).
	Level string `yaml:"level,omitempty"`

	// Format is the log format (text, json).
	Format string `yaml:"format,omitempty"`
}

// SigningConfig contains signing configuration.
type SigningConfig struct {
	// KeyPath is the path to the private signing key.
	KeyPath string `yaml:"key_path,omitempty"`

	// PasswordEnv is the environment variable containing the key password.
	// If not set, user will be prompted interactively.
	PasswordEnv string `yaml:"password_env,omitempty"`
}

// DefaultConfig returns the default configuration.
func DefaultConfig() *Config {
	return &Config{
		Registries: make(map[string]RegistryConfig),
		Deploy: DeployConfig{
			Parallelism: 10,
		},
		Logging: LoggingConfig{
			Level:  "info",
			Format: "text",
		},
	}
}

// DefaultConfigPath returns the default configuration file path.
func DefaultConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".nori", "config.yaml")
}

// Load loads the configuration from a file.
// If no path is provided, it checks for config.yaml first, then config.yml.
func Load(path string) (*Config, error) {
	if path == "" {
		// Try config.yaml first, then config.yml
		path = DefaultConfigPath()
		if _, err := os.Stat(path); os.IsNotExist(err) {
			// Try .yml extension
			ymlPath := strings.TrimSuffix(path, ".yaml") + ".yml"
			if _, err := os.Stat(ymlPath); err == nil {
				path = ymlPath
			}
		}
	}

	// Check if file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		// Return default config if file doesn't exist
		return DefaultConfig(), nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	config := DefaultConfig()
	if err := yaml.Unmarshal(data, config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	return config, nil
}

// Save saves the configuration to a file.
func Save(config *Config, path string) error {
	if path == "" {
		path = DefaultConfigPath()
	}

	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// GetRegistryConfig returns the configuration for a specific registry.
func (c *Config) GetRegistryConfig(registry string) *RegistryConfig {
	if regConfig, ok := c.Registries[registry]; ok {
		return &regConfig
	}
	return nil
}

// IsInsecure returns true if the registry should use insecure connections.
func (c *Config) IsInsecure(registry string) bool {
	if regConfig := c.GetRegistryConfig(registry); regConfig != nil {
		return regConfig.Insecure
	}
	return false
}

// GetStateRepository returns the configured state repository.
// Returns an error if not configured.
func (c *Config) GetStateRepository() (string, error) {
	if c.StateRepository == "" {
		return "", fmt.Errorf("state_repository not configured in ~/.nori/config.yaml")
	}
	return c.StateRepository, nil
}

// GetStateRef returns the full OCI reference for a release state.
// Format: <state_repository>:<release-name>-<version> (flat format for ECR compatibility)
func (c *Config) GetStateRef(releaseName, version string) (string, error) {
	repo, err := c.GetStateRepository()
	if err != nil {
		return "", err
	}
	// Use flat format: repo:releaseName-version
	return fmt.Sprintf("%s:%s-%s", repo, releaseName, version), nil
}
