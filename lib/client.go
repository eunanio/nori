package nori

import (
	"fmt"
	"log/slog"

	"github.com/eunanio/nori/pkg/auth"
	"github.com/eunanio/nori/pkg/config"
	"github.com/eunanio/nori/pkg/oci"
)

// Client is the primary entry point for using nori as a library.
// It provides high-level methods for packaging, pushing, pulling,
// deploying, and managing releases of OCI module artifacts.
//
// Create a Client with NewClient and functional options:
//
//	client, err := nori.NewClient(
//	    nori.WithConfigPath("~/.nori/config.yaml"),
//	    nori.WithLogger(myLogger),
//	)
type Client struct {
	cfg       *config.Config
	logger    *slog.Logger
	ociClient *oci.Client
	credStore *auth.CredentialStore
	insecure  bool
}

// ClientOption configures a Client.
type ClientOption func(*clientOptions)

type clientOptions struct {
	configPath         string
	registryConfigPath string
	logger             *slog.Logger
	insecure           bool
	cfg                *config.Config
}

// WithConfigPath sets the path to the nori configuration file.
// If empty, the default path (~/.nori/config.yaml) is used.
func WithConfigPath(path string) ClientOption {
	return func(o *clientOptions) {
		o.configPath = path
	}
}

// WithRegistryConfigPath overrides the Docker config.json location
// used for registry authentication.
func WithRegistryConfigPath(path string) ClientOption {
	return func(o *clientOptions) {
		o.registryConfigPath = path
	}
}

// WithLogger sets the logger. If nil, slog.Default() is used.
func WithLogger(logger *slog.Logger) ClientOption {
	return func(o *clientOptions) {
		o.logger = logger
	}
}

// WithInsecure allows insecure (plain HTTP) registry connections.
func WithInsecure(insecure bool) ClientOption {
	return func(o *clientOptions) {
		o.insecure = insecure
	}
}

// WithConfig supplies a pre-loaded configuration, skipping file loading.
func WithConfig(cfg *config.Config) ClientOption {
	return func(o *clientOptions) {
		o.cfg = cfg
	}
}

// NewClient creates a new nori Client. It loads configuration,
// initialises the credential store, and creates the OCI client.
func NewClient(opts ...ClientOption) (*Client, error) {
	o := &clientOptions{}
	for _, opt := range opts {
		opt(o)
	}

	logger := o.logger
	if logger == nil {
		logger = slog.Default()
	}

	// Load or use provided config
	var cfg *config.Config
	if o.cfg != nil {
		cfg = o.cfg
	} else {
		var err error
		cfg, err = config.Load(o.configPath)
		if err != nil {
			logger.Warn("failed to load config, using defaults", "error", err)
			cfg = config.DefaultConfig()
		}
	}

	if o.insecure {
		// Merge CLI-level insecure flag into registries if needed
	}

	// Credential store
	dockerConfigPath := o.registryConfigPath
	if dockerConfigPath == "" {
		dockerConfigPath = auth.DefaultDockerConfigPath()
	}
	credStore := auth.NewCredentialStore(dockerConfigPath, logger)

	// OCI client
	ociClient := oci.NewClient(
		oci.WithCredentialStore(credStore),
		oci.WithLogger(logger),
		oci.WithInsecure(o.insecure),
	)

	return &Client{
		cfg:       cfg,
		logger:    logger,
		ociClient: ociClient,
		credStore: credStore,
		insecure:  o.insecure,
	}, nil
}

// Config returns the loaded nori configuration.
func (c *Client) Config() *config.Config {
	return c.cfg
}

// OCI returns the underlying OCI client for advanced use cases.
func (c *Client) OCI() *oci.Client {
	return c.ociClient
}

// CredentialStore returns the underlying credential store.
func (c *Client) CredentialStore() *auth.CredentialStore {
	return c.credStore
}

// stateRepo is a helper that returns the configured state repository
// or an error if not set.
func (c *Client) stateRepo() (string, error) {
	repo, err := c.cfg.GetStateRepository()
	if err != nil {
		return "", fmt.Errorf("state repository not configured: %w", err)
	}
	return repo, nil
}
