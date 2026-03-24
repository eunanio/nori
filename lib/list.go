package nori

import (
	"context"
	"fmt"
	"sort"

	"github.com/google/go-containerregistry/pkg/name"
)

// ListTags returns all tags for a given OCI repository, sorted lexicographically.
func (c *Client) ListTags(ctx context.Context, repository string) ([]string, error) {
	c.logger.Info("listing tags", "repository", repository)

	repo, err := name.NewRepository(repository)
	if err != nil {
		return nil, fmt.Errorf("invalid repository: %w", err)
	}

	tags, err := c.ociClient.ListTags(ctx, repo)
	if err != nil {
		return nil, fmt.Errorf("failed to list tags: %w", err)
	}

	sort.Strings(tags)
	return tags, nil
}
