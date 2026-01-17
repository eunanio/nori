package auth

import (
	"context"
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizeRegistry(t *testing.T) {
	tests := []struct {
		name     string
		registry string
		want     string
	}{
		{
			name:     "plain registry",
			registry: "ghcr.io",
			want:     "ghcr.io",
		},
		{
			name:     "with https prefix",
			registry: "https://ghcr.io",
			want:     "ghcr.io",
		},
		{
			name:     "with http prefix",
			registry: "http://registry.local",
			want:     "registry.local",
		},
		{
			name:     "with trailing slash",
			registry: "ghcr.io/",
			want:     "ghcr.io",
		},
		{
			name:     "with https and trailing slash",
			registry: "https://ghcr.io/",
			want:     "ghcr.io",
		},
		{
			name:     "docker.io converts to index.docker.io",
			registry: "docker.io",
			want:     "index.docker.io",
		},
		{
			name:     "registry with port",
			registry: "registry.example.com:5000",
			want:     "registry.example.com:5000",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, normalizeRegistry(tt.registry))
		})
	}
}

func TestParseAuthEntry_Base64Auth(t *testing.T) {
	cs := NewCredentialStore("", nil)

	// Create base64 encoded username:password
	authStr := base64.StdEncoding.EncodeToString([]byte("myuser:mypassword"))
	entry := AuthEntry{Auth: authStr}

	creds, err := cs.parseAuthEntry(entry)
	require.NoError(t, err)

	assert.Equal(t, "myuser", creds.Username)
	assert.Equal(t, "mypassword", creds.Password)
}

func TestParseAuthEntry_ExplicitCredentials(t *testing.T) {
	cs := NewCredentialStore("", nil)

	entry := AuthEntry{
		Username: "explicit-user",
		Password: "explicit-pass",
	}

	creds, err := cs.parseAuthEntry(entry)
	require.NoError(t, err)

	assert.Equal(t, "explicit-user", creds.Username)
	assert.Equal(t, "explicit-pass", creds.Password)
}

func TestParseAuthEntry_IdentityToken(t *testing.T) {
	cs := NewCredentialStore("", nil)

	entry := AuthEntry{IdentityToken: "my-identity-token"}

	creds, err := cs.parseAuthEntry(entry)
	require.NoError(t, err)

	assert.Equal(t, "my-identity-token", creds.Token)
	assert.Empty(t, creds.Username)
	assert.Empty(t, creds.Password)
}

func TestParseAuthEntry_RegistryToken(t *testing.T) {
	cs := NewCredentialStore("", nil)

	entry := AuthEntry{RegistryToken: "my-registry-token"}

	creds, err := cs.parseAuthEntry(entry)
	require.NoError(t, err)

	assert.Equal(t, "my-registry-token", creds.Token)
}

func TestParseAuthEntry_InvalidBase64(t *testing.T) {
	cs := NewCredentialStore("", nil)

	entry := AuthEntry{Auth: "not-valid-base64!!!"}

	_, err := cs.parseAuthEntry(entry)
	assert.Error(t, err, "should error on invalid base64")
}

func TestParseAuthEntry_InvalidAuthFormat(t *testing.T) {
	cs := NewCredentialStore("", nil)

	// Valid base64 but missing colon separator
	authStr := base64.StdEncoding.EncodeToString([]byte("no-colon-here"))
	entry := AuthEntry{Auth: authStr}

	_, err := cs.parseAuthEntry(entry)
	assert.Error(t, err, "should error on auth without colon")
}

func TestParseAuthEntry_EmptyEntry(t *testing.T) {
	cs := NewCredentialStore("", nil)

	entry := AuthEntry{}

	_, err := cs.parseAuthEntry(entry)
	assert.Error(t, err, "should error on empty entry")
}

func TestGetCredentialsFromEnv_GHCR(t *testing.T) {
	cs := NewCredentialStore("", nil)

	// Set environment variable
	t.Setenv("GITHUB_TOKEN", "test-github-token")

	creds, err := cs.getCredentialsFromEnv("ghcr.io")
	require.NoError(t, err)

	assert.Equal(t, "USERNAME", creds.Username)
	assert.Equal(t, "test-github-token", creds.Password)
}

func TestGetCredentialsFromEnv_GH_TOKEN(t *testing.T) {
	cs := NewCredentialStore("", nil)

	// Set environment variable (fallback to GH_TOKEN)
	os.Unsetenv("GITHUB_TOKEN")
	t.Setenv("GH_TOKEN", "test-gh-token")

	creds, err := cs.getCredentialsFromEnv("ghcr.io")
	require.NoError(t, err)

	assert.Equal(t, "test-gh-token", creds.Password)
}

func TestGetCredentialsFromEnv_NoriPrefix(t *testing.T) {
	cs := NewCredentialStore("", nil)

	t.Setenv("NORI_REGISTRY_USERNAME", "nori-user")
	t.Setenv("NORI_REGISTRY_PASSWORD", "nori-pass")

	creds, err := cs.getCredentialsFromEnv("some-registry.io")
	require.NoError(t, err)

	assert.Equal(t, "nori-user", creds.Username)
	assert.Equal(t, "nori-pass", creds.Password)
}

func TestGetCredentialsFromEnv_RegistrySpecific(t *testing.T) {
	cs := NewCredentialStore("", nil)

	t.Setenv("MY_REGISTRY_IO_USERNAME", "registry-user")
	t.Setenv("MY_REGISTRY_IO_PASSWORD", "registry-pass")

	creds, err := cs.getCredentialsFromEnv("my-registry.io")
	require.NoError(t, err)

	assert.Equal(t, "registry-user", creds.Username)
	assert.Equal(t, "registry-pass", creds.Password)
}

func TestGetCredentialsFromEnv_NotFound(t *testing.T) {
	cs := NewCredentialStore("", nil)

	// Clear any relevant env vars
	os.Unsetenv("GITHUB_TOKEN")
	os.Unsetenv("GH_TOKEN")
	os.Unsetenv("NORI_REGISTRY_USERNAME")
	os.Unsetenv("NORI_REGISTRY_PASSWORD")

	_, err := cs.getCredentialsFromEnv("unknown-registry.io")
	assert.Error(t, err, "should error when no credentials found")
}

func TestDefaultDockerConfigPath(t *testing.T) {
	// Test with DOCKER_CONFIG env var
	tempDir := t.TempDir()
	t.Setenv("DOCKER_CONFIG", tempDir)

	path := DefaultDockerConfigPath()
	expected := filepath.Join(tempDir, "config.json")

	assert.Equal(t, expected, path)
}

func TestDefaultDockerConfigPath_NoEnv(t *testing.T) {
	// Clear DOCKER_CONFIG
	originalValue := os.Getenv("DOCKER_CONFIG")
	os.Unsetenv("DOCKER_CONFIG")
	t.Cleanup(func() {
		if originalValue != "" {
			os.Setenv("DOCKER_CONFIG", originalValue)
		}
	})

	path := DefaultDockerConfigPath()

	if path == "" {
		t.Skip("Could not determine home directory")
	}

	assert.Contains(t, path, ".docker", "should contain .docker")
	assert.Contains(t, path, "config.json", "should contain config.json")
}

func TestNewCredentialStore(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.json")

	cs := NewCredentialStore(configPath, nil)

	assert.Equal(t, configPath, cs.configPath)
	assert.NotNil(t, cs.logger, "logger should not be nil")
}

func TestNewCredentialStore_EmptyPath(t *testing.T) {
	cs := NewCredentialStore("", nil)

	assert.NotEmpty(t, cs.configPath, "should use default path when empty")
}

func TestGetDefaultCredentialHelper(t *testing.T) {
	helper := GetDefaultCredentialHelper()
	// Just verify it returns something or empty (platform dependent)
	// We can't assert specific values as they depend on the runtime OS
	assert.NotPanics(t, func() { _ = helper })
}

func TestLoadDockerConfig_NonExistent(t *testing.T) {
	cs := NewCredentialStore("/nonexistent/path/config.json", nil)

	_, err := cs.loadDockerConfig()
	assert.Error(t, err, "should error for nonexistent file")
}

func TestLoadDockerConfig_ValidConfig(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.json")

	configContent := `{
		"auths": {
			"ghcr.io": {
				"auth": "dGVzdDp0ZXN0"
			}
		},
		"credsStore": "desktop"
	}`

	err := os.WriteFile(configPath, []byte(configContent), 0600)
	require.NoError(t, err, "failed to write test config")

	cs := NewCredentialStore(configPath, nil)

	config, err := cs.loadDockerConfig()
	require.NoError(t, err)

	assert.Equal(t, "desktop", config.CredsStore)
	assert.Contains(t, config.Auths, "ghcr.io")
}

func TestLoadDockerConfig_InvalidJSON(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.json")

	err := os.WriteFile(configPath, []byte("not valid json"), 0600)
	require.NoError(t, err)

	cs := NewCredentialStore(configPath, nil)

	_, err = cs.loadDockerConfig()
	assert.Error(t, err, "should error for invalid JSON")
}

func TestSaveDockerConfig(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "subdir", "config.json")

	cs := NewCredentialStore(configPath, nil)

	config := &DockerConfig{
		Auths: map[string]AuthEntry{
			"ghcr.io": {
				Auth: "dGVzdDp0ZXN0",
			},
		},
	}

	err := cs.saveDockerConfig(config)
	require.NoError(t, err)

	// Verify file exists
	assert.FileExists(t, configPath)

	// Verify content can be loaded back
	loaded, err := cs.loadDockerConfig()
	require.NoError(t, err)
	assert.Contains(t, loaded.Auths, "ghcr.io")
}

func TestStoreCredentials_NewConfig(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.json")

	cs := NewCredentialStore(configPath, nil)

	creds := &Credentials{
		Username: "testuser",
		Password: "testpass",
	}

	err := cs.StoreCredentials("ghcr.io", creds)
	require.NoError(t, err)

	// Verify by loading
	config, err := cs.loadDockerConfig()
	require.NoError(t, err)

	assert.Contains(t, config.Auths, "ghcr.io")
}

func TestRemoveCredentials(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.json")

	// Create initial config
	config := &DockerConfig{
		Auths: map[string]AuthEntry{
			"ghcr.io":         {Auth: "dGVzdDp0ZXN0"},
			"other.io":        {Auth: "b3RoZXI6b3RoZXI="},
			"https://ghcr.io": {Auth: "aHR0cHM6dGVzdA=="},
		},
	}

	cs := NewCredentialStore(configPath, nil)
	err := cs.saveDockerConfig(config)
	require.NoError(t, err)

	// Remove ghcr.io credentials
	err = cs.RemoveCredentials("ghcr.io")
	require.NoError(t, err)

	// Verify removal
	loadedConfig, err := cs.loadDockerConfig()
	require.NoError(t, err)

	assert.NotContains(t, loadedConfig.Auths, "ghcr.io", "should remove ghcr.io")
	assert.NotContains(t, loadedConfig.Auths, "https://ghcr.io", "should remove https://ghcr.io")
	assert.Contains(t, loadedConfig.Auths, "other.io", "should keep other entries")
}

func TestAuthenticator_WithCredentials(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.json")

	// Create config with credentials
	authStr := base64.StdEncoding.EncodeToString([]byte("testuser:testpass"))
	config := &DockerConfig{
		Auths: map[string]AuthEntry{
			"ghcr.io": {Auth: authStr},
		},
	}

	cs := NewCredentialStore(configPath, nil)
	err := cs.saveDockerConfig(config)
	require.NoError(t, err)

	auth, err := cs.Authenticator(context.Background(), "ghcr.io")
	require.NoError(t, err)
	require.NotNil(t, auth)

	// Should return Basic auth
	authConfig, err := auth.Authorization()
	require.NoError(t, err)

	assert.Equal(t, "testuser", authConfig.Username)
}

func TestAuthenticator_NoCredentials(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.json")

	// Create empty config
	config := &DockerConfig{
		Auths: map[string]AuthEntry{},
	}

	cs := NewCredentialStore(configPath, nil)
	err := cs.saveDockerConfig(config)
	require.NoError(t, err)

	// Clear env vars that might provide credentials
	os.Unsetenv("GITHUB_TOKEN")
	os.Unsetenv("GH_TOKEN")
	os.Unsetenv("NORI_REGISTRY_USERNAME")

	auth, err := cs.Authenticator(context.Background(), "unknown-registry.io")
	require.NoError(t, err)

	// Should return anonymous auth without error
	assert.NotNil(t, auth, "should return non-nil authenticator")
}

func TestDockerConfig_Struct(t *testing.T) {
	config := DockerConfig{
		Auths: map[string]AuthEntry{
			"ghcr.io": {
				Auth:     "dGVzdDp0ZXN0",
				Username: "testuser",
				Password: "testpass",
				Email:    "test@example.com",
			},
		},
		CredsStore: "desktop",
		CredHelpers: map[string]string{
			"gcr.io": "gcloud",
		},
	}

	assert.Equal(t, "desktop", config.CredsStore)
	assert.Equal(t, "gcloud", config.CredHelpers["gcr.io"])
	assert.Contains(t, config.Auths, "ghcr.io")
	assert.Equal(t, "testuser", config.Auths["ghcr.io"].Username)
}

func TestCredentials_Struct(t *testing.T) {
	creds := Credentials{
		Username: "user",
		Password: "pass",
		Token:    "token",
	}

	assert.Equal(t, "user", creds.Username)
	assert.Equal(t, "pass", creds.Password)
	assert.Equal(t, "token", creds.Token)
}

func TestAuthEntry_AllFields(t *testing.T) {
	entry := AuthEntry{
		Auth:          "dGVzdDp0ZXN0",
		Username:      "user",
		Password:      "pass",
		Email:         "test@example.com",
		ServerAddress: "https://ghcr.io",
		IdentityToken: "id-token",
		RegistryToken: "reg-token",
	}

	assert.Equal(t, "dGVzdDp0ZXN0", entry.Auth)
	assert.Equal(t, "user", entry.Username)
	assert.Equal(t, "pass", entry.Password)
	assert.Equal(t, "test@example.com", entry.Email)
	assert.Equal(t, "https://ghcr.io", entry.ServerAddress)
	assert.Equal(t, "id-token", entry.IdentityToken)
	assert.Equal(t, "reg-token", entry.RegistryToken)
}
