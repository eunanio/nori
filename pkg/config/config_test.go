package config

import (
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Deploy.Parallelism != 10 {
		t.Errorf("DefaultConfig().Deploy.Parallelism = %d, want %d", cfg.Deploy.Parallelism, 10)
	}

	if cfg.Logging.Level != "info" {
		t.Errorf("DefaultConfig().Logging.Level = %q, want %q", cfg.Logging.Level, "info")
	}
}

func TestLoadSave(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")

	// Create a config
	cfg := DefaultConfig()
	cfg.DefaultRegistry = "ghcr.io"
	cfg.Registries["registry.example.com"] = RegistryConfig{
		Insecure: true,
	}

	// Save it
	if err := Save(cfg, configPath); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Load it back
	loaded, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Verify
	if loaded.DefaultRegistry != cfg.DefaultRegistry {
		t.Errorf("loaded.DefaultRegistry = %q, want %q", loaded.DefaultRegistry, cfg.DefaultRegistry)
	}

	regCfg := loaded.GetRegistryConfig("registry.example.com")
	if regCfg == nil {
		t.Fatal("GetRegistryConfig() returned nil")
	}
	if !regCfg.Insecure {
		t.Error("regCfg.Insecure = false, want true")
	}
}

func TestLoadNonExistent(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "nonexistent.yaml")

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Should return default config
	if cfg.Deploy.Parallelism != 10 {
		t.Errorf("Load() returned non-default config")
	}
}

func TestIsInsecure(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Registries["insecure.example.com"] = RegistryConfig{Insecure: true}
	cfg.Registries["secure.example.com"] = RegistryConfig{Insecure: false}

	tests := []struct {
		registry string
		want     bool
	}{
		{"insecure.example.com", true},
		{"secure.example.com", false},
		{"unknown.example.com", false},
	}

	for _, tt := range tests {
		t.Run(tt.registry, func(t *testing.T) {
			got := cfg.IsInsecure(tt.registry)
			if got != tt.want {
				t.Errorf("IsInsecure(%q) = %v, want %v", tt.registry, got, tt.want)
			}
		})
	}
}
