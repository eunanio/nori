package nori

import (
	"context"
	"fmt"

	"github.com/eunanio/nori/pkg/auth"
)

// Login stores credentials for the given registry.
func (c *Client) Login(_ context.Context, registry string, creds Credentials) error {
	c.logger.Info("logging in to registry", "registry", registry)

	if creds.Username == "" || creds.Password == "" {
		return fmt.Errorf("username and password are required")
	}

	if err := c.credStore.StoreCredentials(registry, &auth.Credentials{
		Username: creds.Username,
		Password: creds.Password,
	}); err != nil {
		return fmt.Errorf("failed to store credentials: %w", err)
	}

	return nil
}

// Logout removes credentials for the given registry.
func (c *Client) Logout(_ context.Context, registry string) error {
	c.logger.Info("logging out from registry", "registry", registry)

	if err := c.credStore.RemoveCredentials(registry); err != nil {
		return fmt.Errorf("failed to remove credentials: %w", err)
	}

	return nil
}
