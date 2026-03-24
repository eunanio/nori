package nori

import (
	"log/slog"
	"testing"

	"github.com/eunanio/nori/pkg/config"
)

func TestNewClient_Defaults(t *testing.T) {
	client, err := NewClient()
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	if client.cfg == nil {
		t.Error("Config() returned nil")
	}
	if client.ociClient == nil {
		t.Error("OCI() returned nil")
	}
	if client.credStore == nil {
		t.Error("CredentialStore() returned nil")
	}
	if client.logger == nil {
		t.Error("logger is nil")
	}
}

func TestNewClient_WithConfig(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.DefaultRegistry = "example.com"

	client, err := NewClient(WithConfig(cfg))
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	if client.Config().DefaultRegistry != "example.com" {
		t.Errorf("DefaultRegistry = %q, want %q", client.Config().DefaultRegistry, "example.com")
	}
}

func TestNewClient_WithLogger(t *testing.T) {
	logger := slog.Default()

	client, err := NewClient(WithLogger(logger))
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	if client.logger != logger {
		t.Error("logger was not set correctly")
	}
}

func TestNewClient_WithInsecure(t *testing.T) {
	client, err := NewClient(WithInsecure(true))
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	if !client.insecure {
		t.Error("insecure should be true")
	}
}

func TestClient_Accessors(t *testing.T) {
	client, err := NewClient()
	if err != nil {
		t.Fatal(err)
	}

	if client.Config() == nil {
		t.Error("Config() should not be nil")
	}
	if client.OCI() == nil {
		t.Error("OCI() should not be nil")
	}
	if client.CredentialStore() == nil {
		t.Error("CredentialStore() should not be nil")
	}
}

func TestClient_StateRepo_NotConfigured(t *testing.T) {
	cfg := config.DefaultConfig()
	client, err := NewClient(WithConfig(cfg))
	if err != nil {
		t.Fatal(err)
	}

	_, err = client.stateRepo()
	if err == nil {
		t.Error("expected error when state repository is not configured")
	}
}
