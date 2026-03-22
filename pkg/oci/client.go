// Package oci provides OCI registry client functionality.
package oci

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/eunanio/nori/pkg/auth"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

// Client is an OCI registry client.
type Client struct {
	credStore *auth.CredentialStore
	insecure  bool
	logger    *slog.Logger
}

// ClientOption is a function that configures a Client.
type ClientOption func(*Client)

// WithInsecure enables insecure (HTTP) connections.
func WithInsecure(insecure bool) ClientOption {
	return func(c *Client) {
		c.insecure = insecure
	}
}

// WithLogger sets the logger for the client.
func WithLogger(logger *slog.Logger) ClientOption {
	return func(c *Client) {
		c.logger = logger
	}
}

// WithCredentialStore sets the credential store for the client.
func WithCredentialStore(store *auth.CredentialStore) ClientOption {
	return func(c *Client) {
		c.credStore = store
	}
}

// NewClient creates a new OCI client.
func NewClient(opts ...ClientOption) *Client {
	c := &Client{
		logger: slog.Default(),
	}

	for _, opt := range opts {
		opt(c)
	}

	if c.credStore == nil {
		c.credStore = auth.NewCredentialStore("", c.logger)
	}

	return c
}

// ParseReference parses an OCI reference string (standalone, without insecure support).
func ParseReference(ref string, opts ...name.Option) (name.Reference, error) {
	return name.ParseReference(ref, opts...)
}

// NameOptions returns the name options for reference parsing.
// When the client is configured as insecure, this includes name.Insecure
// so that go-containerregistry uses HTTP instead of defaulting to HTTPS.
func (c *Client) NameOptions() []name.Option {
	if c.insecure {
		return []name.Option{name.Insecure}
	}
	return nil
}

// ParseReference parses an OCI reference string, applying insecure options when configured.
func (c *Client) ParseReference(ref string) (name.Reference, error) {
	return name.ParseReference(ref, c.NameOptions()...)
}

// NewTag creates a new tag reference, applying insecure options when configured.
func (c *Client) NewTag(tag string) (name.Tag, error) {
	return name.NewTag(tag, c.NameOptions()...)
}

// NewDigest creates a new digest reference, applying insecure options when configured.
func (c *Client) NewDigest(digest string) (name.Digest, error) {
	return name.NewDigest(digest, c.NameOptions()...)
}

// NewRepository creates a new repository reference, applying insecure options when configured.
func (c *Client) NewRepository(repo string) (name.Repository, error) {
	return name.NewRepository(repo, c.NameOptions()...)
}

// RemoteOptions returns the remote options for registry operations.
func (c *Client) RemoteOptions(ctx context.Context, registry string) ([]remote.Option, error) {
	var opts []remote.Option

	// Set up authentication
	authenticator, err := c.credStore.Authenticator(ctx, registry)
	if err != nil {
		c.logger.Debug("failed to get authenticator, using anonymous", "error", err)
		authenticator = authn.Anonymous
	}

	opts = append(opts, remote.WithAuth(authenticator))

	// Handle insecure registries
	if c.insecure {
		c.logger.Warn("using insecure connection", "registry", registry)
		opts = append(opts, remote.WithTransport(insecureTransport()))
	}

	opts = append(opts, remote.WithContext(ctx))

	return opts, nil
}

// insecureTransport returns an HTTP transport that skips TLS certificate verification.
func insecureTransport() http.RoundTripper {
	return &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
}

// RegistryFromRef extracts the registry from a reference.
func RegistryFromRef(ref name.Reference) string {
	return ref.Context().RegistryStr()
}

// RepositoryFromRef extracts the repository from a reference.
func RepositoryFromRef(ref name.Reference) string {
	return ref.Context().RepositoryStr()
}

// TagFromRef extracts the tag from a reference, if present.
func TagFromRef(ref name.Reference) string {
	if tag, ok := ref.(name.Tag); ok {
		return tag.TagStr()
	}
	return ""
}

// DigestFromRef extracts the digest from a reference, if present.
func DigestFromRef(ref name.Reference) string {
	if digest, ok := ref.(name.Digest); ok {
		return digest.DigestStr()
	}
	return ""
}

// ValidateReference validates an OCI reference string.
func ValidateReference(ref string, opts ...name.Option) error {
	// Check basic format
	if ref == "" {
		return fmt.Errorf("reference cannot be empty")
	}

	// Must contain at least one /
	if !strings.Contains(ref, "/") {
		return fmt.Errorf("reference must be in format registry/repository:tag")
	}

	// Parse the reference
	parsed, err := name.ParseReference(ref, opts...)
	if err != nil {
		return fmt.Errorf("invalid reference: %w", err)
	}

	// Validate registry
	registry := parsed.Context().RegistryStr()
	if registry == "" {
		return fmt.Errorf("registry cannot be empty")
	}

	// Validate repository
	repo := parsed.Context().RepositoryStr()
	if repo == "" {
		return fmt.Errorf("repository cannot be empty")
	}

	return nil
}

// ReferenceWithDigest creates a new reference with the given digest.
func ReferenceWithDigest(ref name.Reference, digest string, opts ...name.Option) (name.Reference, error) {
	repo := ref.Context()
	return name.NewDigest(repo.String()+"@"+digest, opts...)
}
