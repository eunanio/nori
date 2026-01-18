// Package auth provides authentication functionality for OCI registries.
package auth

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/google/go-containerregistry/pkg/authn"
)

// CredentialStore manages registry credentials.
type CredentialStore struct {
	configPath string
	logger     *slog.Logger
}

// DockerConfig represents the Docker configuration file structure.
type DockerConfig struct {
	Auths       map[string]AuthEntry `json:"auths,omitempty"`
	CredsStore  string               `json:"credsStore,omitempty"`
	CredHelpers map[string]string    `json:"credHelpers,omitempty"`
}

// AuthEntry represents a single auth entry in the Docker config.
type AuthEntry struct {
	Auth          string `json:"auth,omitempty"`
	Username      string `json:"username,omitempty"`
	Password      string `json:"password,omitempty"`
	Email         string `json:"email,omitempty"`
	ServerAddress string `json:"serverAddress,omitempty"`
	IdentityToken string `json:"identitytoken,omitempty"`
	RegistryToken string `json:"registrytoken,omitempty"`
}

// Credentials holds username and password for registry authentication.
type Credentials struct {
	Username string
	Password string
	Token    string
}

// NewCredentialStore creates a new credential store.
func NewCredentialStore(configPath string, logger *slog.Logger) *CredentialStore {
	if configPath == "" {
		configPath = DefaultDockerConfigPath()
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &CredentialStore{
		configPath: configPath,
		logger:     logger,
	}
}

// DefaultDockerConfigPath returns the default Docker config path.
func DefaultDockerConfigPath() string {
	if dockerConfig := os.Getenv("DOCKER_CONFIG"); dockerConfig != "" {
		return filepath.Join(dockerConfig, "config.json")
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	return filepath.Join(home, ".docker", "config.json")
}

// GetCredentials retrieves credentials for a registry using the following order:
// 1. Credential helpers (credHelpers)
// 2. Default credential store (credsStore)
// 3. Base64 encoded auth in config
// 4. Environment variables
func (cs *CredentialStore) GetCredentials(ctx context.Context, registry string) (*Credentials, error) {
	cs.logger.Debug("getting credentials", "registry", registry)

	// Normalize registry name
	registry = normalizeRegistry(registry)

	// Try loading Docker config
	config, err := cs.loadDockerConfig()
	if err != nil {
		cs.logger.Debug("failed to load docker config, falling back to env vars", "error", err)
		return cs.getCredentialsFromEnv(registry)
	}

	// 1. Try credential helper for this specific registry
	if helper, ok := config.CredHelpers[registry]; ok {
		cs.logger.Debug("using credential helper", "helper", helper, "registry", registry)
		creds, err := cs.getFromCredHelper(helper, registry)
		if err == nil {
			return creds, nil
		}
		cs.logger.Debug("credential helper failed", "error", err)
	}

	// 2. Try default credential store
	if config.CredsStore != "" {
		cs.logger.Debug("using default credential store", "store", config.CredsStore)
		creds, err := cs.getFromCredHelper(config.CredsStore, registry)
		if err == nil {
			return creds, nil
		}
		cs.logger.Debug("default credential store failed", "error", err)
	}

	// 3. Try auth entry in config
	if authEntry, ok := config.Auths[registry]; ok {
		cs.logger.Debug("using auth entry from config")
		return cs.parseAuthEntry(authEntry)
	}

	// Try with https:// prefix
	if authEntry, ok := config.Auths["https://"+registry]; ok {
		cs.logger.Debug("using auth entry from config with https prefix")
		return cs.parseAuthEntry(authEntry)
	}

	// Special handling for Docker Hub
	if registry == "index.docker.io" || registry == "docker.io" {
		for _, r := range []string{"https://index.docker.io/v1/", "https://index.docker.io/v2/"} {
			if authEntry, ok := config.Auths[r]; ok {
				cs.logger.Debug("using Docker Hub auth entry", "registry", r)
				return cs.parseAuthEntry(authEntry)
			}
		}
	}

	// 4. Try environment variables
	cs.logger.Debug("trying environment variables")
	return cs.getCredentialsFromEnv(registry)
}

// loadDockerConfig loads and parses the Docker configuration file.
func (cs *CredentialStore) loadDockerConfig() (*DockerConfig, error) {
	data, err := os.ReadFile(cs.configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read docker config: %w", err)
	}

	var config DockerConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse docker config: %w", err)
	}

	return &config, nil
}

// getFromCredHelper retrieves credentials from a Docker credential helper.
func (cs *CredentialStore) getFromCredHelper(helper, registry string) (*Credentials, error) {
	helperName := "docker-credential-" + helper

	cmd := exec.Command(helperName, "get")
	cmd.Stdin = strings.NewReader(registry)

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("credential helper %s failed: %w", helperName, err)
	}

	var result struct {
		Username string `json:"Username"`
		Secret   string `json:"Secret"`
	}

	if err := json.Unmarshal(output, &result); err != nil {
		return nil, fmt.Errorf("failed to parse credential helper output: %w", err)
	}

	return &Credentials{
		Username: result.Username,
		Password: result.Secret,
	}, nil
}

// parseAuthEntry parses an auth entry from the Docker config.
func (cs *CredentialStore) parseAuthEntry(entry AuthEntry) (*Credentials, error) {
	// Check for identity token (used by some registries)
	if entry.IdentityToken != "" {
		return &Credentials{
			Token: entry.IdentityToken,
		}, nil
	}

	// Check for registry token
	if entry.RegistryToken != "" {
		return &Credentials{
			Token: entry.RegistryToken,
		}, nil
	}

	// Check for explicit username/password
	if entry.Username != "" && entry.Password != "" {
		return &Credentials{
			Username: entry.Username,
			Password: entry.Password,
		}, nil
	}

	// Decode base64 auth
	if entry.Auth != "" {
		decoded, err := base64.StdEncoding.DecodeString(entry.Auth)
		if err != nil {
			return nil, fmt.Errorf("failed to decode auth: %w", err)
		}

		parts := strings.SplitN(string(decoded), ":", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid auth format")
		}

		return &Credentials{
			Username: parts[0],
			Password: parts[1],
		}, nil
	}

	return nil, fmt.Errorf("no valid credentials found in auth entry")
}

// getCredentialsFromEnv retrieves credentials from environment variables.
func (cs *CredentialStore) getCredentialsFromEnv(registry string) (*Credentials, error) {
	// Check for registry-specific environment variables
	envPrefix := strings.ToUpper(strings.ReplaceAll(registry, ".", "_"))
	envPrefix = strings.ReplaceAll(envPrefix, "-", "_")

	// Try registry-specific vars first
	if username := os.Getenv(envPrefix + "_USERNAME"); username != "" {
		password := os.Getenv(envPrefix + "_PASSWORD")
		if password == "" {
			password = os.Getenv(envPrefix + "_TOKEN")
		}
		if password != "" {
			return &Credentials{
				Username: username,
				Password: password,
			}, nil
		}
	}

	// Try generic NORI_ prefix
	if username := os.Getenv("NORI_REGISTRY_USERNAME"); username != "" {
		password := os.Getenv("NORI_REGISTRY_PASSWORD")
		if password != "" {
			return &Credentials{
				Username: username,
				Password: password,
			}, nil
		}
	}

	// Special handling for known registries
	switch {
	case strings.Contains(registry, "ghcr.io"):
		if token := os.Getenv("GITHUB_TOKEN"); token != "" {
			return &Credentials{
				Username: "USERNAME",
				Password: token,
			}, nil
		}
		if token := os.Getenv("GH_TOKEN"); token != "" {
			return &Credentials{
				Username: "USERNAME",
				Password: token,
			}, nil
		}

	case strings.Contains(registry, "ecr") && strings.Contains(registry, "amazonaws.com"):
		// AWS ECR - credentials are handled separately via AWS SDK
		return nil, fmt.Errorf("ECR requires AWS credentials; use 'aws ecr get-login-password' or configure AWS credentials")

	case strings.Contains(registry, "azurecr.io"):
		if clientID := os.Getenv("AZURE_CLIENT_ID"); clientID != "" {
			if secret := os.Getenv("AZURE_CLIENT_SECRET"); secret != "" {
				return &Credentials{
					Username: clientID,
					Password: secret,
				}, nil
			}
		}

	case strings.Contains(registry, "gcr.io") || strings.Contains(registry, "pkg.dev"):
		if keyFile := os.Getenv("GOOGLE_APPLICATION_CREDENTIALS"); keyFile != "" {
			data, err := os.ReadFile(keyFile)
			if err == nil {
				return &Credentials{
					Username: "_json_key",
					Password: string(data),
				}, nil
			}
		}
	}

	return nil, fmt.Errorf("no credentials found for registry %s", registry)
}

// StoreCredentials stores credentials in the Docker config file.
func (cs *CredentialStore) StoreCredentials(registry string, creds *Credentials) error {
	registry = normalizeRegistry(registry)

	// Load existing config or create new one
	config, err := cs.loadDockerConfig()
	if err != nil {
		config = &DockerConfig{
			Auths: make(map[string]AuthEntry),
		}
	}

	if config.Auths == nil {
		config.Auths = make(map[string]AuthEntry)
	}

	// If there's a credential store configured, use it
	if config.CredsStore != "" {
		return cs.storeInCredHelper(config.CredsStore, registry, creds)
	}

	// Otherwise, store in config file
	authStr := base64.StdEncoding.EncodeToString([]byte(creds.Username + ":" + creds.Password))
	config.Auths[registry] = AuthEntry{
		Auth: authStr,
	}

	return cs.saveDockerConfig(config)
}

// storeInCredHelper stores credentials in a credential helper.
func (cs *CredentialStore) storeInCredHelper(helper, registry string, creds *Credentials) error {
	helperName := "docker-credential-" + helper

	input := map[string]string{
		"ServerURL": registry,
		"Username":  creds.Username,
		"Secret":    creds.Password,
	}

	inputBytes, err := json.Marshal(input)
	if err != nil {
		return fmt.Errorf("failed to marshal credentials: %w", err)
	}

	cmd := exec.Command(helperName, "store")
	cmd.Stdin = strings.NewReader(string(inputBytes))

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("credential helper %s failed to store: %w", helperName, err)
	}

	return nil
}

// RemoveCredentials removes credentials for a registry.
func (cs *CredentialStore) RemoveCredentials(registry string) error {
	registry = normalizeRegistry(registry)

	config, err := cs.loadDockerConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// If there's a credential store configured, use it
	if config.CredsStore != "" {
		return cs.removeFromCredHelper(config.CredsStore, registry)
	}

	// Remove from config file
	delete(config.Auths, registry)
	delete(config.Auths, "https://"+registry)

	return cs.saveDockerConfig(config)
}

// removeFromCredHelper removes credentials from a credential helper.
func (cs *CredentialStore) removeFromCredHelper(helper, registry string) error {
	helperName := "docker-credential-" + helper

	cmd := exec.Command(helperName, "erase")
	cmd.Stdin = strings.NewReader(registry)

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("credential helper %s failed to erase: %w", helperName, err)
	}

	return nil
}

// saveDockerConfig saves the Docker configuration to disk.
func (cs *CredentialStore) saveDockerConfig(config *DockerConfig) error {
	// Ensure directory exists
	configDir := filepath.Dir(cs.configPath)
	if err := os.MkdirAll(configDir, 0700); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(cs.configPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	return nil
}

// normalizeRegistry normalizes a registry name.
func normalizeRegistry(registry string) string {
	registry = strings.TrimPrefix(registry, "https://")
	registry = strings.TrimPrefix(registry, "http://")
	registry = strings.TrimSuffix(registry, "/")

	// Handle Docker Hub special cases
	if registry == "docker.io" {
		return "index.docker.io"
	}

	return registry
}

// Authenticator returns an authn.Authenticator for the given registry.
func (cs *CredentialStore) Authenticator(ctx context.Context, registry string) (authn.Authenticator, error) {
	creds, err := cs.GetCredentials(ctx, registry)
	if err != nil {
		// Return anonymous auth if no credentials found
		cs.logger.Debug("no credentials found, using anonymous auth", "registry", registry, "error", err)
		return authn.Anonymous, nil
	}

	if creds.Token != "" {
		return &authn.Bearer{Token: creds.Token}, nil
	}

	return &authn.Basic{
		Username: creds.Username,
		Password: creds.Password,
	}, nil
}

// GetDefaultCredentialHelper returns the default credential helper for the current platform.
func GetDefaultCredentialHelper() string {
	switch runtime.GOOS {
	case "darwin":
		return "osxkeychain"
	case "windows":
		return "wincred"
	case "linux":
		// Check if pass or secretservice is available
		if _, err := exec.LookPath("docker-credential-pass"); err == nil {
			return "pass"
		}
		if _, err := exec.LookPath("docker-credential-secretservice"); err == nil {
			return "secretservice"
		}
		return ""
	default:
		return ""
	}
}

// GetCredentialHelper returns the credential helper to use for OCI registries.
// It first checks the Docker config's credsStore setting (which reflects the user's
// actual Docker configuration), then falls back to platform-specific defaults.
// This handles cases like:
// - Docker Desktop on Windows using "desktop" instead of "wincred"
// - WSL using "wincred" from the Docker config
// - Custom credential helpers configured by the user
func GetCredentialHelper() string {
	// First check Docker config's credsStore (most accurate reflection of user's setup)
	configPath := DefaultDockerConfigPath()
	if configPath != "" {
		if data, err := os.ReadFile(configPath); err == nil {
			var config DockerConfig
			if err := json.Unmarshal(data, &config); err == nil && config.CredsStore != "" {
				return config.CredsStore
			}
		}
	}

	// Fall back to platform defaults
	return GetDefaultCredentialHelper()
}